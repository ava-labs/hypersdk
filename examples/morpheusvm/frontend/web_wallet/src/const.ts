export const SNAP_ID = 'local:http://localhost:8080'
export const DEVELOPMENT_MODE = window.location.hostname === 'localhost'
export const HRP = 'morpheus'
export const COIN_SYMBOL = "RED"
export const DECIMAL_PLACES = 9
export const MAX_TRANSFER_FEE = 10000000n

export const FAUCET_HOST = DEVELOPMENT_MODE ? 'http://localhost:8765' : ''
export const API_HOST = DEVELOPMENT_MODE ? 'http://localhost:9650' : ''