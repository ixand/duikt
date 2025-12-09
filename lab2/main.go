package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Transaction represents a simple transaction structure (JSON)
type Transaction struct {
	ID        string  `json:"id"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
	// Note: For simplicity we omit real signatures. In production include signatures.
}

// Block represents a block in the chain
type Block struct {
	Index        int           `json:"index"`
	Timestamp    int64         `json:"timestamp"`
	Transactions []Transaction `json:"transactions"`
	Nonce        int64         `json:"nonce"`
	PreviousHash string        `json:"previous_hash"`
	Hash         string        `json:"hash"`
}

// Blockchain holds the chain and synchronization primitives
type Blockchain struct {
	Chain  []Block
	Mutex  sync.Mutex
	Difficulty int // number of leading zeros required in hex hash prefix
}

// Mempool stores pending transactions
type Mempool struct {
	Txs   []Transaction
	Mutex sync.Mutex
}

var blockchain *Blockchain
var mempool *Mempool

func init() {
	rand.Seed(time.Now().UnixNano())
	blockchain = &Blockchain{
		Chain: make([]Block, 0),
		Difficulty: 3, // adjust difficulty (3 = moderate)
	}
	mempool = &Mempool{
		Txs: make([]Transaction, 0),
	}
	// create genesis block
	genesis := Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []Transaction{},
		Nonce:        0,
		PreviousHash: "0",
	}
	genesis.Hash = calculateHash(genesis)
	blockchain.Chain = append(blockchain.Chain, genesis)
}

// calculateHash computes SHA256 hash of block fields (as hex)
func calculateHash(b Block) string {
	record := strconv.Itoa(b.Index) + strconv.FormatInt(b.Timestamp, 10) + transactionsToString(b.Transactions) + strconv.FormatInt(b.Nonce, 10) + b.PreviousHash
	h := sha256.Sum256([]byte(record))
	return hex.EncodeToString(h[:])
}

func transactionsToString(txs []Transaction) string {
	if len(txs) == 0 {
		return ""
	}
	b, _ := json.Marshal(txs)
	return string(b)
}

// createTransaction creates transaction with ID and timestamp
func createTransaction(from, to string, amount float64) Transaction {
	t := Transaction{
		ID:        randomID(),
		From:      from,
		To:        to,
		Amount:    amount,
		Timestamp: time.Now().Unix(),
	}
	return t
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// addTransactionToMempool appends tx in thread-safe way
func addTransactionToMempool(tx Transaction) {
	mempool.Mutex.Lock()
	defer mempool.Mutex.Unlock()
	mempool.Txs = append(mempool.Txs, tx)
}

// flushMempool returns up to n txs and removes them from mempool
func flushMempool(n int) []Transaction {
	mempool.Mutex.Lock()
	defer mempool.Mutex.Unlock()

	if n <= 0 || n > len(mempool.Txs) {
		n = len(mempool.Txs)
	}
	selected := make([]Transaction, n)
	copy(selected, mempool.Txs[:n])
	// remove selected
	mempool.Txs = mempool.Txs[n:]
	return selected
}

// proofOfWork finds nonce so that hash has required difficulty (leading zeros in hex)
func proofOfWork(b Block, difficulty int) (int64, string) {
	var hash string
	var nonce int64 = 0
	targetPrefix := strings.Repeat("0", difficulty)
	for {
		b.Nonce = nonce
		hash = calculateHash(b)
		if strings.HasPrefix(hash, targetPrefix) {
			return nonce, hash
		}
		nonce++
		// simple break guard (not necessary in small demo)
		if nonce%1_000_000 == 0 {
			// allow other goroutines to run
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// addBlock adds block to chain (thread-safe)
func (bc *Blockchain) addBlock(b Block) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.Chain = append(bc.Chain, b)
}

// getLastBlock returns last block
func (bc *Blockchain) getLastBlock() Block {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	return bc.Chain[len(bc.Chain)-1]
}

// getBalance computes simple balance by iterating chain and mempool (optional)
func (bc *Blockchain) getBalance(address string) float64 {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	var bal float64 = 0
	for _, blk := range bc.Chain {
		for _, tx := range blk.Transactions {
			if tx.To == address {
				bal += tx.Amount
			}
			if tx.From == address {
				bal -= tx.Amount
			}
		}
	}
	// include mempool (pending)
	mempool.Mutex.Lock()
	for _, tx := range mempool.Txs {
		if tx.To == address {
			bal += tx.Amount
		}
		if tx.From == address {
			bal -= tx.Amount
		}
	}
	mempool.Mutex.Unlock()
	return bal
}

func main() {
	r := gin.Default()

	// Create new transaction
	// POST /transactions
	// body: { "from":"alice", "to":"bob", "amount": 1.23 }
	r.POST("/transactions", func(c *gin.Context) {
		var body struct {
			From   string  `json:"from" binding:"required"`
			To     string  `json:"to" binding:"required"`
			Amount float64 `json:"amount" binding:"required,gt=0"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tx := createTransaction(body.From, body.To, body.Amount)
		addTransactionToMempool(tx)
		c.JSON(http.StatusCreated, gin.H{"status": "ok", "tx": tx})
	})

	// View mempool
	// GET /mempool
	r.GET("/mempool", func(c *gin.Context) {
		mempool.Mutex.Lock()
		defer mempool.Mutex.Unlock()
		c.JSON(http.StatusOK, gin.H{"pending": mempool.Txs})
	})

	// Mine a block
	// POST /mine
	// optional JSON body: { "max_txs": 10, "miner": "miner_address" }
	r.POST("/mine", func(c *gin.Context) {
		var body struct {
			MaxTxs int    `json:"max_txs"`
			Miner  string `json:"miner"`
		}
		// default parameters
		body.MaxTxs = 10
		body.Miner = "miner-reward"
		_ = c.BindJSON(&body)

		// take transactions from mempool
		selected := flushMempool(body.MaxTxs)

		// add coinbase reward tx
		reward := Transaction{
			ID:        randomID(),
			From:      "network",
			To:        body.Miner,
			Amount:    1.0, // fixed reward for demo
			Timestamp: time.Now().Unix(),
		}
		selected = append([]Transaction{reward}, selected...)

		last := blockchain.getLastBlock()

		newBlock := Block{
			Index:        last.Index + 1,
			Timestamp:    time.Now().Unix(),
			Transactions: selected,
			PreviousHash: last.Hash,
		}

		nonce, hash := proofOfWork(newBlock, blockchain.Difficulty)
		newBlock.Nonce = nonce
		newBlock.Hash = hash

		blockchain.addBlock(newBlock)

		c.JSON(http.StatusOK, gin.H{
			"status": "mined",
			"block":  newBlock,
		})
	})

	// View blockchain
	// GET /chain
	r.GET("/chain", func(c *gin.Context) {
		blockchain.Mutex.Lock()
		defer blockchain.Mutex.Unlock()
		c.JSON(http.StatusOK, gin.H{"length": len(blockchain.Chain), "chain": blockchain.Chain})
	})

	// Get balance for address
	// GET /balance/:address
	r.GET("/balance/:address", func(c *gin.Context) {
		addr := c.Param("address")
		bal := blockchain.getBalance(addr)
		c.JSON(http.StatusOK, gin.H{"address": addr, "balance": bal})
	})

	// Health
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	log.Println("Starting server at :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
