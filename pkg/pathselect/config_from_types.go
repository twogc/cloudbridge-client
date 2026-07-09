package pathselect

import "github.com/twogc/cloudbridge-client/pkg/types"

// ConfigFromTypes maps types.PathSelectConfig into pathselect.Config.
func ConfigFromTypes(c types.PathSelectConfig) Config {
	return Config{
		Enabled:          c.Enabled,
		Order:            append([]string(nil), c.Order...),
		ProbeTimeout:     c.ProbeTimeout,
		LadderTimeout:    c.LadderTimeout,
		HealthInterval:   c.HealthInterval,
		FailoverCooldown: c.FailoverCooldown,
		SoftFail:         c.SoftFail,
	}.Normalize()
}

// NewSelectorFromConfig builds a LadderSelector from full client config.
func NewSelectorFromConfig(cfg *types.Config) *LadderSelector {
	if cfg == nil {
		return NewLadderSelector(DefaultConfig(), nil)
	}
	ps := ConfigFromTypes(cfg.PathSelect)
	return NewLadderSelector(ps, NewDefaultPaths(cfg))
}
