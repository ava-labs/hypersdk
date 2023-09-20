# Token Wallet

## Configuration
If you want to override the default configuration, place a `config.json` file at `~/.token-wallet/config.json`.

## Live Development
To run in live development mode, run `./scripts/dev.sh` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Release
To build a distributable package for MacOS, run `./scripts/build.sh`.
