# Middleware

This package contains a middleware implementation for a generic `HyperlaneApp`.
The middleware allows to hook
before and after the execution of the application's `Handle` method.

## Usage

To wrap an Hyperlane application with a middleware, two components are required:

- The Hyperlane core keeper.
- An Hyperlane application, like Warp.

Once the Cosmos SDK application has been built,
it is possible to build and register the middleware.
Assuming we want to provide hook functionalities
around the Warp application for the collateral token, we can do as follow.

Import the required types:

```go
import (
	"context"
	"testing"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
	corekeeper "github.com/bcp-innovations/hyperlane-cosmos/x/core/keeper"
	warp "github.com/bcp-innovations/hyperlane-cosmos/x/warp"
	warpkeeper "github.com/bcp-innovations/hyperlane-cosmos/x/warp/keeper"
	"github.com/bcp-innovations/hyperlane-cosmos/x/warp/middleware"
	warptypes "github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
)
```

Now we have to create a concrete type implementing the `HandleHookI` interface. It is possible to
use the simple hook type already provided in the package:

```go

	hook := middleware.NewHandleHook(
		middleware.WithPostHandleFn(func(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
			// DO whatever
			return nil
		}),
		middleware.WithPreHandleHookFn(func(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
			// DO whatever
			return nil
		}),
	)
```

For more complex applications, a custom type can be created.
It is also possible to use another Cosmos SDK module as middleware,
as long as it implements the required methods.
At this point, it is possible to create the middleware. In the example below,
we are going to create two wrapped middlewares around the warp application:

```go

	warpKeeper := &warpkeeper.Keeper{}
	coreKeeper := &corekeeper.Keeper{}

	middlewareInternal := middleware.NewMiddleware(warpKeeper, hook)
	middlewareExternal := middleware.NewMiddleware(middlewareExternal, hook)
```

The only thing left to do, is to register the application in the core Hyperlane keeper:

```go

	warpApps := []warp.WarpApp{
		{TokenType: warptypes.HYP_TOKEN_TYPE_COLLATERAL, App: middlewareExternal},
		{TokenType: warptypes.HYP_TOKEN_TYPE_SYNTHETIC, App: warpKeeper},
	}

	warp.RegisterWarpApp(coreKeeper, warpApps...)
```
