# Middleware

This package contains a middleware implementation for a generic `HyperlaneApp`.
The middleware allows to hook
before and after the execution of the application's `Handle` method.

## Usage

To wrap an Hyperlane application with a middleware, two components are required:

- The Hyperlane core keeper.
- An Hyperlane application, like Warp.

Once the Cosmos SDK application has been built, it is possible to register
