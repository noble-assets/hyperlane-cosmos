# Middleware

This package contains a middleware implementation for a generic `HyperlaneApp`.
Middleware allows integrators to add custom logic
before and after the execution of an application's `Handle` method.

## Usage

To wrap a Hyperlane application with middleware, two components are required:

- The Hyperlane core keeper.
- A Hyperlane application, like Warp.

Once the Cosmos SDK application has been built,
it is possible to build and register the middleware.
Assuming we want to provide hook functionalities around the Warp application for collateral tokens,
we can do the following.

Import the required types:

```go
import (
	"context"
	"testing"

	"github.com/bcp-innovations/hyperlane-cosmos/util"
	warpkeeper "github.com/bcp-innovations/hyperlane-cosmos/x/warp/keeper"
  "github.com/bcp-innovations/hyperlane-cosmos/x/middleware"
	warptypes "github.com/bcp-innovations/hyperlane-cosmos/x/warp/types"
)
```

Now we have to create a concrete value implementing the `HandleHook` interface. It is possible to
use the simple hook type already provided:

```go
hook := middleware.NewHandleHook(
  middleware.WithPostHandleFn(func(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
    // DO whatever
    return nil
  }),
  middleware.WithPreHandleFn(func(ctx context.Context, mailboxID util.HexAddress, message util.HyperlaneMessage) error {
    // DO whatever
    return nil
  }),
)
```

For more complex applications, a custom type can be created.
It is also possible to use another Cosmos SDK module as middleware,
as long as it implements the required methods.
At this point, it is possible to create the middleware. In the example below,
we are going to create two wrapped middlewares around the Warp application:

```go
warpKeeper := &warpkeeper.Keeper{}
coreKeeper := &corekeeper.Keeper{}

middlewareInternal, err := middleware.NewMiddleware(warpKeeper, hook)
if err != nil {
  // handle error
}
middlewareExternal, err := middleware.NewMiddleware(middlewareInternal, hook)
if err != nil {
  // handle error
}
```

The only thing left to do, is to register the application in the core Hyperlane keeper:

```go
// Register the warp keeper with hook with the middleware for the collateral type.
coreKeeper.RegisterApp(warptypes.HYP_TOKEN_TYPE_COLLATERAL, middlewareExternal)

// Register the warp keeper without hook for the synthetic type.
coreKeeper.RegisterApp(warptypes.HYP_TOKEN_TYPE_SYNTHETIC, warpKeeper)
```
