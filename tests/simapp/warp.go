package simapp

import (
	warp "github.com/bcp-innovations/hyperlane-cosmos/x/warp"
)

// RegisterDefaultWarpAppsOpt is a post build option used to register the default warp
// applications.
func RegisterDefaultWarpAppsOpt() PostBuildOpt {
	return func(a *App) {
		defaultWarpApps := warp.DefaultWarpApps(&a.WarpKeeper)
		a.RegisterWarpApps(defaultWarpApps...)
	}
}

// RegisterWarpAppsOpt is a post build option used to register warp
// applications.
func RegisterWarpAppsOpt(warpApps ...warp.WarpApp) PostBuildOpt {
	return func(a *App) {
		a.RegisterWarpApps(warpApps...)
	}
}

// RegisterWarpApps allows to register warp applications on the hyperlane
// core keeper of the main app.
func (app *App) RegisterWarpApps(warpApps ...warp.WarpApp) {
	for _, warpApp := range warpApps {
		app.HyperlaneKeeper.RegisterApp(uint8(warpApp.TokenType), warpApp.Handler)
	}
}
