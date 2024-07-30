export const DEVELOPMENT_MODE = typeof window === 'undefined' || window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
export const HRP = 'morpheus'
export const COIN_SYMBOL = "RED"
export const DECIMAL_PLACES = 9
export const MAX_TRANSFER_FEE = 10000000n

export const FAUCET_HOST = DEVELOPMENT_MODE ? 'http://localhost:8765' : ''
export const API_HOST = DEVELOPMENT_MODE ? 'http://localhost:9650' : ''

export const SNAP_ID = DEVELOPMENT_MODE ? 'local:http://localhost:8080' : 'npm:sample-metamask-snap-for-hypersdk'
