package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BlockHlukhov — структура блоку
type BlockHlukhov struct {
	Index     int
	Timestamp string
	Data      string
	PrevHash  string
	Hash      string
	Nonce     int
}

// BlockchainHlukhov — структура блокчейну
type BlockchainHlukhov struct {
	Blocks []BlockHlukhov
}

// calculateHashHlukhov — обчислює хеш блоку
func calculateHashHlukhov(block BlockHlukhov) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + block.Data + block.PrevHash + strconv.Itoa(block.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// addBlockHlukhov — додає новий блок у ланцюг
func (bc *BlockchainHlukhov) addBlockHlukhov(data string) {
	prevBlock := bc.Blocks[len(bc.Blocks)-1]
	newBlock := BlockHlukhov{
		Index:     len(bc.Blocks),
		Timestamp: time.Now().String(),
		Data:      data,
		PrevHash:  prevBlock.Hash,
		Nonce:     0,
	}

	// "Майнінг": шукаємо хеш, який закінчується на "09"
	for {
		newBlock.Hash = calculateHashHlukhov(newBlock)
		if strings.HasSuffix(newBlock.Hash, "09") {
			break
		}
		newBlock.Nonce++
	}

	bc.Blocks = append(bc.Blocks, newBlock)
}

// createGenesisBlockHlukhov — створює генезис-блок
func createGenesisBlockHlukhov() BlockHlukhov {
	genesis := BlockHlukhov{
		Index:     0,
		Timestamp: time.Now().String(),
		Data:      "Genesis Block",
		PrevHash:  "Hlukhov", // прізвище латиницею
		Nonce:     18092004,  // день, місяць, рік народження
	}
	genesis.Hash = calculateHashHlukhov(genesis)
	return genesis
}

// main
func main() {
	var blockchain BlockchainHlukhov
	blockchain.Blocks = append(blockchain.Blocks, createGenesisBlockHlukhov())

	blockchain.addBlockHlukhov("My first block")
	blockchain.addBlockHlukhov("My second block")

	for _, block := range blockchain.Blocks {
		fmt.Printf("Index: %d\nTimespan: %s\nHash: %s\nData: %s\n prevHash: %s \nNonce: %d\n\n", block.Index, block.Timestamp, block.Hash, block.Data, block.PrevHash, block.Nonce)
	}
}
