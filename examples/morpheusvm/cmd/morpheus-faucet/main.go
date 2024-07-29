package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/time/rate"

	"context"
	"fmt"
	"log"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

const amtStr = "10.00"

func transferCoins(to string) (string, error) {
	toAddr, err := codec.ParseAddressBech32(lconsts.HRP, to)
	if err != nil {
		return "", fmt.Errorf("failed to parse to address: %w", err)
	}

	amt, err := utils.ParseBalance(amtStr, consts.Decimals)
	if err != nil {
		return "", fmt.Errorf("failed to parse amount: %w", err)
	}

	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	if err != nil {
		return "", fmt.Errorf("failed to load private key: %w", err)
	}

	priv := ed25519.PrivateKey(privBytes)
	factory := auth.NewED25519Factory(priv)

	url := "http://localhost:9650/ext/bc/morpheusvm"
	cli := rpc.NewJSONRPCClient(url)

	networkId, subnetId, chainId, err := cli.Network(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get network info: %w", err)
	}
	fmt.Println(networkId, subnetId, chainId)

	lcli := lrpc.NewJSONRPCClient(url, networkId, chainId)
	balanceBefore, err := lcli.Balance(context.TODO(), to)
	if err != nil {
		return "", fmt.Errorf("failed to get balance: %w", err)
	}
	fmt.Printf("Balance before: %s\n", utils.FormatBalance(balanceBefore, consts.Decimals))

	// Check if balance is greater than 1.000
	threshold, _ := utils.ParseBalance("1.000", consts.Decimals)
	if balanceBefore > threshold {
		fmt.Printf("Balance is already greater than 1.000, no transfer needed\n")
		return "Balance is already greater than 1.000, no transfer needed", nil
	}

	parser, err := lcli.Parser(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get parser: %w", err)
	}

	submit, _, _, err := cli.GenerateTransaction(
		context.TODO(),
		parser,
		[]chain.Action{&actions.Transfer{
			To:    toAddr,
			Value: amt,
		}},
		factory,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate transaction: %w", err)
	}

	err = submit(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to submit transaction: %w", err)
	}

	err = lcli.WaitForBalance(context.TODO(), to, amt)
	if err != nil {
		return "", fmt.Errorf("failed to wait for balance: %w", err)
	}

	return "Coins transferred successfully to " + to, nil
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/faucet/{address}", handleFaucetRequest).Methods("GET", "POST")

	// Create a new CORS handler
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	// Wrap the router with the CORS handler
	handler := c.Handler(r)

	fmt.Println("Starting faucet server on :8765")
	if err := http.ListenAndServe(":8765", handler); err != nil {
		log.Fatal(err)
	}
}

var (
	ipLimiters = make(map[string]*rate.Limiter)
	mu         sync.Mutex
)

func handleFaucetRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get client IP
	clientIP := r.RemoteAddr

	// Check rate limit
	if !getRateLimiter(clientIP).Allow() {
		http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
		return
	}

	vars := mux.Vars(r)
	address := vars["address"]
	if address == "" {
		http.Error(w, "Address not provided", http.StatusBadRequest)
		return
	}

	message, err := transferCoins(address)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to transfer coins: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, message)
}

func getRateLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := ipLimiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(30*time.Second), 5)
		ipLimiters[ip] = limiter
	}

	return limiter
}
