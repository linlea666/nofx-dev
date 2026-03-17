package trader

import (
	"nofx/market"
	"nofx/store"
	"time"
)

// ============================================================================
// Task 6: Regime Level Classification
// ============================================================================

// RegimeThresholds holds configurable breakpoints for regime classification.
// Zero values are replaced with crypto defaults.
type RegimeThresholds struct {
	NarrowBBWidth   float64
	StandardBBWidth float64
	WideBBWidth     float64
	NarrowATRPct    float64
	StandardATRPct  float64
	WideATRPct      float64
}

// ResolveRegimeThresholds builds thresholds from GridStrategyConfig,
// falling back to crypto defaults for any zero-valued field.
func ResolveRegimeThresholds(cfg *store.GridStrategyConfig) RegimeThresholds {
	t := RegimeThresholds{
		NarrowBBWidth: 2.0, StandardBBWidth: 3.0, WideBBWidth: 4.0,
		NarrowATRPct: 1.0, StandardATRPct: 2.0, WideATRPct: 3.0,
	}
	if cfg == nil {
		return t
	}
	if cfg.NarrowBBWidth > 0 {
		t.NarrowBBWidth = cfg.NarrowBBWidth
	}
	if cfg.StandardBBWidth > 0 {
		t.StandardBBWidth = cfg.StandardBBWidth
	}
	if cfg.WideBBWidth > 0 {
		t.WideBBWidth = cfg.WideBBWidth
	}
	if cfg.NarrowATRPct > 0 {
		t.NarrowATRPct = cfg.NarrowATRPct
	}
	if cfg.StandardATRPct > 0 {
		t.StandardATRPct = cfg.StandardATRPct
	}
	if cfg.WideATRPct > 0 {
		t.WideATRPct = cfg.WideATRPct
	}
	return t
}

// classifyRegimeLevel determines the regime level based on market indicators
// bollingerWidth: Bollinger band width as percentage
// atr14Pct: ATR14 as percentage of current price
func classifyRegimeLevel(bollingerWidth, atr14Pct float64) market.RegimeLevel {
	return classifyRegimeLevelWithThresholds(bollingerWidth, atr14Pct, RegimeThresholds{
		NarrowBBWidth: 2.0, StandardBBWidth: 3.0, WideBBWidth: 4.0,
		NarrowATRPct: 1.0, StandardATRPct: 2.0, WideATRPct: 3.0,
	})
}

// classifyRegimeLevelWithThresholds uses configurable thresholds for regime classification
func classifyRegimeLevelWithThresholds(bollingerWidth, atr14Pct float64, t RegimeThresholds) market.RegimeLevel {
	if bollingerWidth < t.NarrowBBWidth && atr14Pct < t.NarrowATRPct {
		return market.RegimeLevelNarrow
	}
	if bollingerWidth <= t.StandardBBWidth && atr14Pct <= t.StandardATRPct {
		return market.RegimeLevelStandard
	}
	if bollingerWidth <= t.WideBBWidth && atr14Pct <= t.WideATRPct {
		return market.RegimeLevelWide
	}
	return market.RegimeLevelVolatile
}

// getRegimeLeverageLimit returns the effective leverage limit for a regime level
func getRegimeLeverageLimit(level market.RegimeLevel, config *store.GridConfigModel) int {
	switch level {
	case market.RegimeLevelNarrow:
		if config.NarrowRegimeLeverage > 0 {
			return config.NarrowRegimeLeverage
		}
		return 2
	case market.RegimeLevelStandard:
		if config.StandardRegimeLeverage > 0 {
			return config.StandardRegimeLeverage
		}
		return 4
	case market.RegimeLevelWide:
		if config.WideRegimeLeverage > 0 {
			return config.WideRegimeLeverage
		}
		return 3
	case market.RegimeLevelVolatile:
		if config.VolatileRegimeLeverage > 0 {
			return config.VolatileRegimeLeverage
		}
		return 2
	default:
		return 2 // Conservative default
	}
}

// getRegimePositionLimit returns the position limit percentage for a regime level
func getRegimePositionLimit(level market.RegimeLevel, config *store.GridConfigModel) float64 {
	switch level {
	case market.RegimeLevelNarrow:
		if config.NarrowRegimePositionPct > 0 {
			return config.NarrowRegimePositionPct
		}
		return 40.0
	case market.RegimeLevelStandard:
		if config.StandardRegimePositionPct > 0 {
			return config.StandardRegimePositionPct
		}
		return 70.0
	case market.RegimeLevelWide:
		if config.WideRegimePositionPct > 0 {
			return config.WideRegimePositionPct
		}
		return 60.0
	case market.RegimeLevelVolatile:
		if config.VolatileRegimePositionPct > 0 {
			return config.VolatileRegimePositionPct
		}
		return 40.0
	default:
		return 40.0 // Conservative default
	}
}

// ============================================================================
// Task 7: Breakout Detection
// ============================================================================

// detectBoxBreakout checks if price has broken out of any box level
// Returns the highest breakout level and direction
func detectBoxBreakout(box *market.BoxData) (market.BreakoutLevel, string) {
	if box == nil {
		return market.BreakoutNone, ""
	}

	price := box.CurrentPrice

	// Check long box first (highest priority)
	if price > box.LongUpper {
		return market.BreakoutLong, "up"
	}
	if price < box.LongLower {
		return market.BreakoutLong, "down"
	}

	// Check mid box
	if price > box.MidUpper {
		return market.BreakoutMid, "up"
	}
	if price < box.MidLower {
		return market.BreakoutMid, "down"
	}

	// Check short box
	if price > box.ShortUpper {
		return market.BreakoutShort, "up"
	}
	if price < box.ShortLower {
		return market.BreakoutShort, "down"
	}

	return market.BreakoutNone, ""
}

// ============================================================================
// Task 8: Breakout Confirmation Logic
// ============================================================================

const BreakoutConfirmRequired = 3 // 3 candles to confirm breakout

// BreakoutState tracks the current breakout state
type BreakoutState struct {
	Level        market.BreakoutLevel
	Direction    string
	ConfirmCount int
	StartTime    time.Time
}

// confirmBreakout updates breakout state and returns true if breakout is confirmed
func confirmBreakout(state *BreakoutState, currentLevel market.BreakoutLevel, direction string) bool {
	// If price returned to box, reset state
	if currentLevel == market.BreakoutNone {
		state.ConfirmCount = 0
		state.Level = market.BreakoutNone
		state.Direction = ""
		return false
	}

	// If same breakout continues, increment count
	if state.Level == currentLevel && state.Direction == direction {
		state.ConfirmCount++
	} else {
		// New breakout, reset count
		state.Level = currentLevel
		state.Direction = direction
		state.ConfirmCount = 1
		state.StartTime = time.Now()
	}

	return state.ConfirmCount >= BreakoutConfirmRequired
}

// ============================================================================
// Task 9: Breakout Handler
// ============================================================================

// BreakoutAction represents the action to take on breakout
type BreakoutAction int

const (
	BreakoutActionNone BreakoutAction = iota
	BreakoutActionReducePosition // Short box breakout: reduce to 50%
	BreakoutActionPauseGrid      // Mid box breakout: pause grid + cancel orders
	BreakoutActionCloseAll       // Long box breakout: pause + cancel + close all
)

// getBreakoutAction returns the appropriate action for a breakout level
func getBreakoutAction(level market.BreakoutLevel) BreakoutAction {
	switch level {
	case market.BreakoutShort:
		return BreakoutActionReducePosition
	case market.BreakoutMid:
		return BreakoutActionPauseGrid
	case market.BreakoutLong:
		return BreakoutActionCloseAll
	default:
		return BreakoutActionNone
	}
}

// ============================================================================
// Task 10: Grid Direction Adjustment
// ============================================================================

const (
	// BreakoutActionAdjustDirection adjusts grid direction based on breakout
	BreakoutActionAdjustDirection BreakoutAction = 4
)

// determineGridDirection determines the new grid direction based on box breakout
// currentDirection: the current grid direction
// breakoutLevel: which box level has been broken (short/mid/long)
// direction: breakout direction ("up" or "down")
// Returns: the new grid direction
func determineGridDirection(box *market.BoxData, currentDirection market.GridDirection, breakoutLevel market.BreakoutLevel, direction string) market.GridDirection {
	if box == nil {
		return currentDirection
	}

	price := box.CurrentPrice

	switch breakoutLevel {
	case market.BreakoutShort:
		// Short box breakout: bias direction
		// Still within mid box, so not a full trend yet
		if direction == "up" {
			return market.GridDirectionLongBias
		}
		return market.GridDirectionShortBias

	case market.BreakoutMid:
		// Mid box breakout: full direction
		// More significant move, commit fully
		if direction == "up" {
			return market.GridDirectionLong
		}
		return market.GridDirectionShort

	case market.BreakoutLong:
		// Long box breakout: handled by existing emergency logic
		// Return current direction, let existing handlers take over
		return currentDirection

	case market.BreakoutNone:
		// No breakout - check if we should recover toward neutral
		return determineRecoveryDirection(price, box, currentDirection)

	default:
		return currentDirection
	}
}

// determineRecoveryDirection determines if grid direction should recover toward neutral
// This implements the gradual recovery logic: long → long_bias → neutral ← short_bias ← short
func determineRecoveryDirection(price float64, box *market.BoxData, currentDirection market.GridDirection) market.GridDirection {
	// Check if price is back inside the short box
	insideShortBox := price >= box.ShortLower && price <= box.ShortUpper

	if !insideShortBox {
		// Still outside short box, maintain current direction
		return currentDirection
	}

	// Price is inside short box, start recovery toward neutral
	switch currentDirection {
	case market.GridDirectionLong:
		// Full long → bias long
		return market.GridDirectionLongBias
	case market.GridDirectionLongBias:
		// Bias long → neutral
		return market.GridDirectionNeutral
	case market.GridDirectionShort:
		// Full short → bias short
		return market.GridDirectionShortBias
	case market.GridDirectionShortBias:
		// Bias short → neutral
		return market.GridDirectionNeutral
	default:
		return currentDirection
	}
}

// getBreakoutActionWithDirection returns the appropriate action for a breakout level
// when direction adjustment is enabled
func getBreakoutActionWithDirection(level market.BreakoutLevel, enableDirectionAdjust bool) BreakoutAction {
	if !enableDirectionAdjust {
		// Fall back to original behavior
		return getBreakoutAction(level)
	}

	switch level {
	case market.BreakoutShort:
		// Short box breakout with direction adjustment: adjust direction instead of reducing position
		return BreakoutActionAdjustDirection
	case market.BreakoutMid:
		// Mid box breakout with direction adjustment: adjust to full direction
		return BreakoutActionAdjustDirection
	case market.BreakoutLong:
		// Long box breakout: always trigger emergency handling
		return BreakoutActionCloseAll
	default:
		return BreakoutActionNone
	}
}

// shouldRecoverDirection checks if the current grid direction should start recovering toward neutral
func shouldRecoverDirection(box *market.BoxData, currentDirection market.GridDirection) bool {
	if box == nil || currentDirection == market.GridDirectionNeutral {
		return false
	}

	price := box.CurrentPrice
	// Check if price is back inside the short box
	return price >= box.ShortLower && price <= box.ShortUpper
}
