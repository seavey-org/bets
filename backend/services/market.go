package services

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/codyseavey/bets/models"
)

type MarketService struct {
	db *gorm.DB
}

func NewMarketService(db *gorm.DB) *MarketService {
	return &MarketService{db: db}
}

type CreateMarketRequest struct {
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	Outcomes    []string `json:"outcomes" binding:"required,min=2"`
	ClosesAt    string   `json:"closes_at"` // RFC3339 optional
	Liquidity   int      `json:"liquidity"` // Initial liquidity in points (deducted from creator), default 100
}

type TradeRequest struct {
	OutcomeID string  `json:"outcome_id" binding:"required"`
	Shares    float64 `json:"shares" binding:"required,gt=0"`
}

// CreateMarket creates a new prediction market with CPMM liquidity pool.
// The creator seeds the pool with initial liquidity (points are deducted).
func (s *MarketService) CreateMarket(groupID, userID string, req CreateMarketRequest) (*models.Market, error) {
	if len(req.Outcomes) < 2 {
		return nil, fmt.Errorf("market requires at least 2 outcomes")
	}

	liquidity := req.Liquidity
	if liquidity <= 0 {
		liquidity = 100
	}

	// Initial shares per outcome = liquidity (keeps math clean)
	initialShares := float64(liquidity)

	var closesAt *time.Time
	if req.ClosesAt != "" {
		t, err := time.Parse(time.RFC3339, req.ClosesAt)
		if err != nil {
			return nil, fmt.Errorf("invalid closes_at format (use RFC3339): %w", err)
		}
		if t.Before(time.Now()) {
			return nil, fmt.Errorf("closes_at must be in the future")
		}
		closesAt = &t
	}

	tx := s.db.Begin()

	// Deduct liquidity from creator's group balance
	var member models.GroupMember
	if err := tx.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("not a member of this group")
	}
	if member.PointsBalance < liquidity {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient points for liquidity (have %d, need %d)", member.PointsBalance, liquidity)
	}

	member.PointsBalance -= liquidity
	if err := tx.Save(&member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Log liquidity deduction
	logEntry := &models.PointsLog{
		ID:      uuid.New().String(),
		GroupID: groupID,
		UserID:  userID,
		Amount:  -liquidity,
		Type:    models.PointsLogMarketBuy,
		Note:    fmt.Sprintf("Seeded market liquidity (%d pts)", liquidity),
	}
	if err := tx.Create(logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	market := &models.Market{
		ID:          uuid.New().String(),
		GroupID:     groupID,
		Title:       req.Title,
		Description: req.Description,
		Status:      models.MarketStatusOpen,
		CreatedBy:   userID,
		ClosesAt:    closesAt,
	}

	if err := tx.Create(market).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create market: %w", err)
	}

	for _, label := range req.Outcomes {
		outcome := &models.MarketOutcome{
			ID:       uuid.New().String(),
			MarketID: market.ID,
			Label:    label,
			Shares:   initialShares,
		}
		if err := tx.Create(outcome).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create outcome: %w", err)
		}
		market.Outcomes = append(market.Outcomes, *outcome)
	}

	logEntry.ReferenceID = market.ID
	if err := tx.Save(logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.populatePrices(market)
	return market, nil
}

// GetMarket returns a market with outcomes, prices, and stats.
func (s *MarketService) GetMarket(marketID string) (*models.Market, error) {
	var market models.Market
	err := s.db.
		Preload("Outcomes").
		Preload("Creator").
		First(&market, "id = ?", marketID).Error
	if err != nil {
		return nil, err
	}
	s.populatePrices(&market)
	s.populateStats(&market)
	return &market, nil
}

// GetGroupMarkets returns all markets for a group.
func (s *MarketService) GetGroupMarkets(groupID, status string) ([]models.Market, error) {
	query := s.db.Where("group_id = ?", groupID).Preload("Outcomes").Preload("Creator").Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var markets []models.Market
	if err := query.Find(&markets).Error; err != nil {
		return nil, err
	}

	for i := range markets {
		s.populatePrices(&markets[i])
		s.populateStats(&markets[i])
	}

	return markets, nil
}

// BuyShares executes a buy trade using the CPMM.
// The user pays points to receive shares of a specific outcome.
func (s *MarketService) BuyShares(marketID, userID string, req TradeRequest) (*models.Trade, error) {
	tx := s.db.Begin()

	var market models.Market
	if err := tx.Preload("Outcomes").First(&market, "id = ?", marketID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("market not found")
	}
	if market.Status != models.MarketStatusOpen {
		tx.Rollback()
		return nil, fmt.Errorf("market is not open for trading")
	}
	if market.ClosesAt != nil && time.Now().After(*market.ClosesAt) {
		tx.Rollback()
		return nil, fmt.Errorf("market has closed for trading")
	}

	// Find the target outcome
	var targetIdx int = -1
	for i, o := range market.Outcomes {
		if o.ID == req.OutcomeID {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		tx.Rollback()
		return nil, fmt.Errorf("invalid outcome for this market")
	}

	// Calculate cost using CPMM
	cost := s.calculateBuyCost(market.Outcomes, targetIdx, req.Shares)
	pointsCost := int(math.Ceil(cost)) // Round up to not leak value

	if pointsCost <= 0 {
		tx.Rollback()
		return nil, fmt.Errorf("trade too small")
	}

	// Check user balance
	var member models.GroupMember
	if err := tx.Where("group_id = ? AND user_id = ?", market.GroupID, userID).First(&member).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("not a member of this group")
	}
	if member.PointsBalance < pointsCost {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient points (have %d, need %d)", member.PointsBalance, pointsCost)
	}

	// Deduct points
	member.PointsBalance -= pointsCost
	if err := tx.Save(&member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update CPMM pool: remove shares from the target outcome
	market.Outcomes[targetIdx].Shares -= req.Shares
	if market.Outcomes[targetIdx].Shares < 0.001 {
		tx.Rollback()
		return nil, fmt.Errorf("not enough liquidity for this trade")
	}
	if err := tx.Save(&market.Outcomes[targetIdx]).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update user's share position
	if err := s.addSharePosition(tx, marketID, userID, req.OutcomeID, req.Shares); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Calculate price (cost per share)
	price := cost / req.Shares

	// Record trade
	trade := &models.Trade{
		ID:         uuid.New().String(),
		MarketID:   marketID,
		UserID:     userID,
		OutcomeID:  req.OutcomeID,
		Side:       "buy",
		Shares:     req.Shares,
		PointsCost: pointsCost,
		Price:      price,
	}
	if err := tx.Create(trade).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Points log
	logEntry := &models.PointsLog{
		ID:          uuid.New().String(),
		GroupID:     market.GroupID,
		UserID:      userID,
		Amount:      -pointsCost,
		Type:        models.PointsLogMarketBuy,
		ReferenceID: trade.ID,
		Note:        fmt.Sprintf("Bought %.1f shares of \"%s\" in \"%s\"", req.Shares, market.Outcomes[targetIdx].Label, market.Title),
	}
	if err := tx.Create(logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return trade, nil
}

// SellShares executes a sell trade. User returns shares and receives points.
func (s *MarketService) SellShares(marketID, userID string, req TradeRequest) (*models.Trade, error) {
	tx := s.db.Begin()

	var market models.Market
	if err := tx.Preload("Outcomes").First(&market, "id = ?", marketID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("market not found")
	}
	if market.Status != models.MarketStatusOpen {
		tx.Rollback()
		return nil, fmt.Errorf("market is not open for trading")
	}

	// Find target outcome
	var targetIdx int = -1
	for i, o := range market.Outcomes {
		if o.ID == req.OutcomeID {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		tx.Rollback()
		return nil, fmt.Errorf("invalid outcome for this market")
	}

	// Check user has enough shares
	var position models.SharePosition
	err := tx.Where("market_id = ? AND user_id = ? AND outcome_id = ?", marketID, userID, req.OutcomeID).First(&position).Error
	if err != nil || position.Shares < req.Shares {
		tx.Rollback()
		if err != nil {
			return nil, fmt.Errorf("you have no shares of this outcome")
		}
		return nil, fmt.Errorf("insufficient shares (have %.1f, want to sell %.1f)", position.Shares, req.Shares)
	}

	// Calculate payout using CPMM
	payout := s.calculateSellPayout(market.Outcomes, targetIdx, req.Shares)
	pointsPayout := int(math.Floor(payout)) // Round down to not leak value

	if pointsPayout <= 0 {
		tx.Rollback()
		return nil, fmt.Errorf("trade too small")
	}

	// Credit points
	var member models.GroupMember
	if err := tx.Where("group_id = ? AND user_id = ?", market.GroupID, userID).First(&member).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("not a member of this group")
	}
	member.PointsBalance += pointsPayout
	if err := tx.Save(&member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update CPMM pool: add shares back to the target outcome
	market.Outcomes[targetIdx].Shares += req.Shares
	if err := tx.Save(&market.Outcomes[targetIdx]).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update share position
	if err := s.addSharePosition(tx, marketID, userID, req.OutcomeID, -req.Shares); err != nil {
		tx.Rollback()
		return nil, err
	}

	price := payout / req.Shares

	trade := &models.Trade{
		ID:         uuid.New().String(),
		MarketID:   marketID,
		UserID:     userID,
		OutcomeID:  req.OutcomeID,
		Side:       "sell",
		Shares:     req.Shares,
		PointsCost: -pointsPayout,
		Price:      price,
	}
	if err := tx.Create(trade).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	logEntry := &models.PointsLog{
		ID:          uuid.New().String(),
		GroupID:     market.GroupID,
		UserID:      userID,
		Amount:      pointsPayout,
		Type:        models.PointsLogMarketSell,
		ReferenceID: trade.ID,
		Note:        fmt.Sprintf("Sold %.1f shares of \"%s\" in \"%s\"", req.Shares, market.Outcomes[targetIdx].Label, market.Title),
	}
	if err := tx.Create(logEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return trade, nil
}

// ResolveMarket resolves a market by declaring the winning outcome.
// Holders of winning shares receive 1 point per share. Losing shares are worthless.
func (s *MarketService) ResolveMarket(marketID, winningOutcomeID, userID string, isAdmin bool) error {
	tx := s.db.Begin()

	var market models.Market
	if err := tx.First(&market, "id = ?", marketID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("market not found")
	}
	if market.Status != models.MarketStatusOpen && market.Status != models.MarketStatusClosed {
		tx.Rollback()
		return fmt.Errorf("market cannot be resolved (status: %s)", market.Status)
	}
	if market.CreatedBy != userID && !isAdmin {
		tx.Rollback()
		return fmt.Errorf("only market creator or group admin can resolve")
	}

	// Verify winning outcome
	var outcome models.MarketOutcome
	if err := tx.First(&outcome, "id = ? AND market_id = ?", winningOutcomeID, marketID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("invalid winning outcome")
	}

	// Pay out winning share holders: 1 point per share
	var winningPositions []models.SharePosition
	if err := tx.Where("market_id = ? AND outcome_id = ? AND shares > 0", marketID, winningOutcomeID).Find(&winningPositions).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, pos := range winningPositions {
		payout := int(math.Floor(pos.Shares))
		if payout <= 0 {
			continue
		}

		result := tx.Model(&models.GroupMember{}).
			Where("group_id = ? AND user_id = ?", market.GroupID, pos.UserID).
			Update("points_balance", gorm.Expr("points_balance + ?", payout))
		if result.Error != nil {
			tx.Rollback()
			return result.Error
		}

		logEntry := &models.PointsLog{
			ID:          uuid.New().String(),
			GroupID:     market.GroupID,
			UserID:      pos.UserID,
			Amount:      payout,
			Type:        models.PointsLogMarketWin,
			ReferenceID: marketID,
			Note:        fmt.Sprintf("Won %d pts from market \"%s\" (%.0f shares of \"%s\")", payout, market.Title, pos.Shares, outcome.Label),
		}
		if err := tx.Create(logEntry).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	now := time.Now()
	if err := tx.Model(&market).Updates(map[string]interface{}{
		"status":             models.MarketStatusResolved,
		"winning_outcome_id": winningOutcomeID,
		"resolved_at":        now,
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// CancelMarket cancels a market and refunds all traders based on their net spend.
func (s *MarketService) CancelMarket(marketID, userID string, isAdmin bool) error {
	tx := s.db.Begin()

	var market models.Market
	if err := tx.First(&market, "id = ?", marketID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("market not found")
	}
	if market.Status == models.MarketStatusResolved || market.Status == models.MarketStatusCancelled {
		tx.Rollback()
		return fmt.Errorf("market is already %s", market.Status)
	}
	if market.CreatedBy != userID && !isAdmin {
		tx.Rollback()
		return fmt.Errorf("only market creator or group admin can cancel")
	}

	// Refund each user their net spend (sum of all trade costs)
	type UserRefund struct {
		UserID    string
		NetSpend  int
	}
	var refunds []UserRefund
	if err := tx.Model(&models.Trade{}).
		Select("user_id, SUM(points_cost) as net_spend").
		Where("market_id = ?", marketID).
		Group("user_id").
		Find(&refunds).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, r := range refunds {
		if r.NetSpend <= 0 {
			continue // Already net positive from sells, nothing to refund
		}
		result := tx.Model(&models.GroupMember{}).
			Where("group_id = ? AND user_id = ?", market.GroupID, r.UserID).
			Update("points_balance", gorm.Expr("points_balance + ?", r.NetSpend))
		if result.Error != nil {
			tx.Rollback()
			return result.Error
		}

		logEntry := &models.PointsLog{
			ID:          uuid.New().String(),
			GroupID:     market.GroupID,
			UserID:      r.UserID,
			Amount:      r.NetSpend,
			Type:        models.PointsLogMarketRefund,
			ReferenceID: marketID,
			Note:        fmt.Sprintf("Refund from cancelled market \"%s\"", market.Title),
		}
		if err := tx.Create(logEntry).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Also refund creator's initial liquidity
	var liquidityLog models.PointsLog
	if err := tx.Where("reference_id = ? AND type = ? AND note LIKE ?", marketID, models.PointsLogMarketBuy, "Seeded market liquidity%").First(&liquidityLog).Error; err == nil {
		refundAmt := -liquidityLog.Amount // Was stored as negative
		if refundAmt > 0 {
			tx.Model(&models.GroupMember{}).
				Where("group_id = ? AND user_id = ?", market.GroupID, market.CreatedBy).
				Update("points_balance", gorm.Expr("points_balance + ?", refundAmt))

			tx.Create(&models.PointsLog{
				ID:          uuid.New().String(),
				GroupID:     market.GroupID,
				UserID:      market.CreatedBy,
				Amount:      refundAmt,
				Type:        models.PointsLogMarketRefund,
				ReferenceID: marketID,
				Note:        fmt.Sprintf("Liquidity refund from cancelled market \"%s\"", market.Title),
			})
		}
	}

	if err := tx.Model(&market).Update("status", models.MarketStatusCancelled).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GetUserPositions returns all share positions for a user in a group.
func (s *MarketService) GetUserPositions(groupID, userID string) ([]models.SharePosition, error) {
	var positions []models.SharePosition
	err := s.db.
		Joins("JOIN markets ON markets.id = share_positions.market_id").
		Where("markets.group_id = ? AND share_positions.user_id = ? AND share_positions.shares > 0", groupID, userID).
		Preload("Outcome").
		Find(&positions).Error
	return positions, err
}

// GetMarketTrades returns trade history for a market.
func (s *MarketService) GetMarketTrades(marketID string) ([]models.Trade, error) {
	var trades []models.Trade
	err := s.db.Where("market_id = ?", marketID).
		Preload("User").
		Preload("Outcome").
		Order("created_at DESC").
		Find(&trades).Error
	return trades, err
}

// GetMarketGroupID returns the group ID for a market.
func (s *MarketService) GetMarketGroupID(marketID string) (string, error) {
	var market models.Market
	if err := s.db.Select("group_id").First(&market, "id = ?", marketID).Error; err != nil {
		return "", err
	}
	return market.GroupID, nil
}

// --- CPMM Math ---

// calculateBuyCost calculates the cost in points to buy `shares` of outcome at targetIdx.
//
// Uses the complete-sets CPMM formulation:
// User pays C points, which mints C shares of every outcome (added to pool),
// then extracts `shares` shares of the target outcome.
// Invariant: product of all pool shares = k (before and after).
func (s *MarketService) calculateBuyCost(outcomes []models.MarketOutcome, targetIdx int, shares float64) float64 {
	return s.costViaCompleteSets(outcomes, targetIdx, shares)
}

// costViaCompleteSets calculates cost using the complete-sets CPMM formulation.
// User pays C points, which mints C shares of every outcome (added to pool),
// then extracts `shares` shares of the target outcome.
// Invariant: product of all pool shares = k (before and after).
//
// For binary: (a + C - s)(b + C) = a*b, solve for C (positive root).
// For n outcomes: product_i(q_i + C + delta_i) = k, where delta_i = -s for target, 0 otherwise.
func (s *MarketService) costViaCompleteSets(outcomes []models.MarketOutcome, targetIdx int, shares float64) float64 {
	n := len(outcomes)
	
	if n == 2 {
		// Binary market: solve quadratic
		a := outcomes[0].Shares
		b := outcomes[1].Shares
		k := a * b
		
		var target, other float64
		if targetIdx == 0 {
			target = a
			other = b
		} else {
			target = b
			other = a
		}
		
		// (target + C - shares)(other + C) = k
		// C^2 + C*(target - shares + other) + (target - shares)*other - k = 0
		// C^2 + C*(target + other - shares) + (target*other - shares*other - k) = 0
		// Since k = target*other:
		// C^2 + C*(target + other - shares) - shares*other = 0
		_ = target
		A := 1.0
		B := (outcomes[0].Shares + outcomes[1].Shares - shares)
		C := -shares * other
		
		discriminant := B*B - 4*A*C
		if discriminant < 0 {
			return math.MaxFloat64
		}
		
		cost := (-B + math.Sqrt(discriminant)) / (2 * A)
		if cost < 0 {
			return math.MaxFloat64
		}
		return cost
	}
	
	// For n > 2: use numerical method (binary search)
	k := 1.0
	for _, o := range outcomes {
		k *= o.Shares
	}
	
	// Binary search for C where product(q_i + C + delta_i) = k
	lo, hi := 0.0, 10000.0
	for iter := 0; iter < 100; iter++ {
		mid := (lo + hi) / 2
		prod := 1.0
		for i, o := range outcomes {
			if i == targetIdx {
				prod *= (o.Shares + mid - shares)
			} else {
				prod *= (o.Shares + mid)
			}
		}
		if prod < k {
			lo = mid
		} else {
			hi = mid
		}
	}
	
	return (lo + hi) / 2
}

// calculateSellPayout calculates the points received for selling `shares` of outcome at targetIdx.
// This is the reverse of buying: user returns shares to pool, receives points.
func (s *MarketService) calculateSellPayout(outcomes []models.MarketOutcome, targetIdx int, shares float64) float64 {
	n := len(outcomes)
	k := 1.0
	for _, o := range outcomes {
		k *= o.Shares
	}
	
	if n == 2 {
		// Selling s shares of target: user puts s back, extracts C complete sets.
		// After: (target + shares - C) * (other - C) = k
		// For selling: target gets +shares, all pools get -C (removing complete sets)
		a := outcomes[0].Shares
		b := outcomes[1].Shares
		
		var target, other float64
		if targetIdx == 0 {
			target = a
			other = b
		} else {
			target = b
			other = a
		}
		
		// (target + shares - C)(other - C) = k = target * other
		// C^2 - C*(target + shares + other) + (target + shares)*other - target*other = 0
		// C^2 - C*(target + shares + other) + shares*other = 0
		A := 1.0
		B := -(target + shares + other)
		C_coeff := shares * other
		
		discriminant := B*B - 4*A*C_coeff
		if discriminant < 0 {
			return 0
		}
		
		// We want the smaller positive root
		root1 := (-B - math.Sqrt(discriminant)) / (2 * A)
		root2 := (-B + math.Sqrt(discriminant)) / (2 * A)
		
		// Take the smaller positive root (payout shouldn't exceed pool)
		payout := root1
		if payout <= 0 {
			payout = root2
		}
		if payout <= 0 || payout > other {
			return 0
		}
		return payout
	}
	
	// For n > 2: binary search
	lo, hi := 0.0, 10000.0
	for iter := 0; iter < 100; iter++ {
		mid := (lo + hi) / 2
		prod := 1.0
		for i, o := range outcomes {
			val := o.Shares - mid // Remove mid complete sets from all
			if i == targetIdx {
				val += shares // Add returned shares to target
			}
			if val <= 0 {
				prod = 0
				break
			}
			prod *= val
		}
		if prod > k {
			lo = mid
		} else {
			hi = mid
		}
	}
	
	return (lo + hi) / 2
}

// GetPrice returns the current price for one share of a given outcome.
// Price = cost to buy an infinitesimally small amount.
// For CPMM: price_i = product_j!=i(q_j) / sum_i(product_j!=i(q_j))
func (s *MarketService) GetPrice(outcomes []models.MarketOutcome, idx int) float64 {
	if len(outcomes) == 0 {
		return 0
	}

	// Calculate product of all shares except each outcome
	inverseProducts := make([]float64, len(outcomes))
	totalProduct := 1.0
	for _, o := range outcomes {
		totalProduct *= o.Shares
	}

	sum := 0.0
	for i, o := range outcomes {
		if o.Shares == 0 {
			inverseProducts[i] = math.MaxFloat64
		} else {
			inverseProducts[i] = totalProduct / o.Shares
		}
		sum += inverseProducts[i]
	}

	if sum == 0 {
		return 0
	}

	return inverseProducts[idx] / sum
}

func (s *MarketService) populatePrices(market *models.Market) {
	for i := range market.Outcomes {
		market.Outcomes[i].Price = s.GetPrice(market.Outcomes, i)
	}
}

func (s *MarketService) populateStats(market *models.Market) {
	var totalVolume int64
	var tradeCount int64
	s.db.Model(&models.Trade{}).Where("market_id = ?", market.ID).Select("COALESCE(SUM(ABS(points_cost)), 0)").Scan(&totalVolume)
	s.db.Model(&models.Trade{}).Where("market_id = ?", market.ID).Count(&tradeCount)
	market.TotalVolume = int(totalVolume)
	market.TradeCount = int(tradeCount)
}

func (s *MarketService) addSharePosition(tx *gorm.DB, marketID, userID, outcomeID string, shares float64) error {
	var position models.SharePosition
	err := tx.Where("market_id = ? AND user_id = ? AND outcome_id = ?", marketID, userID, outcomeID).First(&position).Error
	
	if err == gorm.ErrRecordNotFound {
		if shares < 0 {
			return fmt.Errorf("cannot have negative share position")
		}
		position = models.SharePosition{
			ID:        uuid.New().String(),
			MarketID:  marketID,
			UserID:    userID,
			OutcomeID: outcomeID,
			Shares:    shares,
		}
		return tx.Create(&position).Error
	}
	if err != nil {
		return err
	}

	position.Shares += shares
	if position.Shares < -0.001 {
		return fmt.Errorf("cannot have negative share position")
	}
	if position.Shares < 0 {
		position.Shares = 0 // Fix floating point dust
	}
	return tx.Save(&position).Error
}
