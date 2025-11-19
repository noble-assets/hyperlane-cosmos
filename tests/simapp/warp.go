package simapp

import (
	warp "github.com/bcp-innovations/hyperlane-cosmos/x/warp"
)

func (app *App) RegisterWarpApps(warpApps ...warp.WarpApp) {
	for _, warpApp := range warpApps {
		app.HyperlaneKeeper.RegisterApp(uint8(warpApp.TokenType), warpApp.App)
	}
}
