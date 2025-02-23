package block

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"learn-blockchain/utils"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MINING_DIFFICULTY = 3
	MINING_SENDER     = "THE BLOCKCHAIN"
	MINING_REWARD     = 1.0
	MINING_TIMER_SEC  = 20

	BLOCKCHAIN_PORT_RANGE_START       = 5000
	BLOCKCHAIN_PORT_RANGE_END         = 5003
	NEIGHBOR_IP_RANGE_START           = 0
	NEIGHBOR_IP_RANGE_END             = 1
	BLOCKCHAIN_NEIGHBOR_SYNC_TIME_SEC = 20
)

type Block struct {
	timestamp    int64
	nonce        int
	previousHash [32]byte
	transactions []*Transaction
}

func CreateNewBlock(nonce int, previosHash [32]byte, transactions []*Transaction) *Block {
	block := new(Block)
	block.timestamp = time.Now().UnixNano()
	block.nonce = nonce
	block.previousHash = previosHash
	block.transactions = transactions
	return block
}

func (b *Block) PreviousHash() [32]byte {
	return b.previousHash
}

func (b *Block) Nonce() int {
	return b.nonce
}

func (b *Block) Transactions() []*Transaction {
	return b.transactions
}

func (block *Block) Print() {
	fmt.Printf("timestamp       %d\n", block.timestamp)
	fmt.Printf("nonce           %d\n", block.nonce)
	fmt.Printf("previous_hash   %x\n", block.previousHash)

	for _, t := range block.transactions {
		t.Print()
	}
}

func (block *Block) Hash() [32]byte {
	m, _ := json.Marshal(block)
	// fmt.Println(m)
	return sha256.Sum256([]byte(m))
}

func (block *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp    int64          `json:"timestamp"`
		Nonce        int            `json:"nonce"`
		PreviosHash  string         `json:"previous_hash"`
		Transactions []*Transaction `json:"transactions"`
	}{
		Timestamp:    block.timestamp,
		Nonce:        block.nonce,
		PreviosHash:  fmt.Sprintf("%x", block.previousHash),
		Transactions: block.transactions,
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var previousHash string
	v := &struct {
		Timestamp    *int64          `json:"timestamp"`
		Nonce        *int            `json:"nonce"`
		PreviousHash *string         `json:"previous_hash"`
		Transactions *[]*Transaction `json:"transactions"`
	}{
		Timestamp:    &b.timestamp,
		Nonce:        &b.nonce,
		PreviousHash: &previousHash,
		Transactions: &b.transactions,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ph, _ := hex.DecodeString(*v.PreviousHash)
	copy(b.previousHash[:], ph[:32])
	return nil
}

type Blockchain struct {
	transactionPool   []*Transaction
	chain             []*Block
	blockchainAddress string
	port              uint16
	mux               sync.Mutex

	neighbors    []string
	muxNeighbors sync.Mutex
}

func NewBlockchain(blockchainAddress string, port uint16) *Blockchain {
	block := &Block{}
	blockchain := new(Blockchain)
	blockchain.blockchainAddress = blockchainAddress
	blockchain.port = port
	blockchain.CreateBlock(0, block.Hash())

	return blockchain
}

func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}

func (bc *Blockchain) Run() {
	if bc == nil {
		log.Fatal("Blockchain is nil, cannot run!")
	}
	bc.StartSyncNeighbors()
	bc.ResolveConflicts()
}

func (bc *Blockchain) SetNeighbors() {
	bc.neighbors = utils.FindNeighbors(
		utils.GetHost(), bc.port,
		NEIGHBOR_IP_RANGE_START, NEIGHBOR_IP_RANGE_END,
		BLOCKCHAIN_PORT_RANGE_START, BLOCKCHAIN_PORT_RANGE_END)
	log.Printf("%v", bc.neighbors)
}

func (bc *Blockchain) SyncNeighbors() {
	log.Printf("Syncing neighbors for blockchain at port %d", bc.port)
	bc.muxNeighbors.Lock()
	defer bc.muxNeighbors.Unlock()
	bc.SetNeighbors()
}

func (bc *Blockchain) StartSyncNeighbors() {
	bc.SyncNeighbors()
	_ = time.AfterFunc(time.Second*BLOCKCHAIN_NEIGHBOR_SYNC_TIME_SEC, bc.StartSyncNeighbors)
}

func (bc *Blockchain) TransactionPool() []*Transaction {
	return bc.transactionPool
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = bc.transactionPool[:0]
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*Block `json:"chain"`
	}{
		Blocks: bc.chain,
	})
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*Block `json:"chain"`
	}{
		Blocks: &bc.chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (blockchain *Blockchain) CreateBlock(nonce int, previosHash [32]byte) *Block {
	block := CreateNewBlock(nonce, previosHash, blockchain.transactionPool)
	blockchain.chain = append(blockchain.chain, block)
	blockchain.transactionPool = []*Transaction{}
	for _, n := range blockchain.neighbors {
		endpoint := fmt.Sprintf("http://%s/transactions", n)
		client := &http.Client{}
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}

	return block
}

func (blockchain *Blockchain) LastBlock() *Block {
	return blockchain.chain[len(blockchain.chain)-1]
}

func (blockchain *Blockchain) Print() {
	for i, block := range blockchain.chain {
		fmt.Printf("%s Chain %d %s\n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		block.Print()
	}

	fmt.Printf("%s\n", strings.Repeat("*", 60))
}

func (bc *Blockchain) CreateTransaction(sender string, recipient string, value float32,
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	isTransacted := bc.AddTransaction(sender, recipient, value, senderPublicKey, s)

	if isTransacted {
		log.Printf("Neighbors before syncing transaction: %v", bc.neighbors)
		for _, n := range bc.neighbors {
			publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(),
				senderPublicKey.Y.Bytes())
			signatureStr := s.String()
			bt := &TransactionRequest{
				&sender, &recipient, &publicKeyStr, &value, &signatureStr}
			m, _ := json.Marshal(bt)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("http://%s/transactions", n)
			client := &http.Client{}
			req, _ := http.NewRequest("PUT", endpoint, buf)
			resp, _ := client.Do(req)
			log.Printf("%v", resp)
		}
	}

	return isTransacted
}

func (bc *Blockchain) AddTransaction(sender string, recipient string, value float32, senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	t := NewTransaction(sender, recipient, value)

	if sender == MINING_SENDER {
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}

	if bc.VerifyTransactionSignature(senderPublicKey, s, t) {
		/*
			if bc.CalculateTotalAmount(sender) < value {
				log.Println("ERROR: Not enough balance in a wallet")
				return false
			}
		*/

		bc.transactionPool = append(bc.transactionPool, t)
		return true
	} else {
		log.Print("Error : Verify transaction")
	}

	return false
}

func (bc *Blockchain) VerifyTransactionSignature(
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256([]byte(m))
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
}

func (bc *Blockchain) CopyTransactionPool() []*Transaction {
	transactions := make([]*Transaction, len(bc.transactionPool))
	for _, t := range bc.transactionPool {
		transactions = append(transactions,
			NewTransaction(t.senderBlockchainAddress,
				t.recipientBlockchainAddress,
				t.value))
	}
	return transactions
}

func (bc *Blockchain) ValidProof(nonce int, previousHash [32]byte, transactions []*Transaction, difficulty int) bool {
	zeros := strings.Repeat("0", difficulty)
	guessBlock := Block{0, nonce, previousHash, transactions}
	guessHashStr := fmt.Sprintf("%x", guessBlock.Hash())
	return guessHashStr[:difficulty] == zeros
}

func (bc *Blockchain) ProofOfWork() int {
	transactions := bc.CopyTransactionPool()
	previousHash := bc.LastBlock().Hash()
	nonce := 0
	for !bc.ValidProof(nonce, previousHash, transactions, MINING_DIFFICULTY) {
		nonce += 1
	}
	return nonce
}

func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// if len(bc.transactionPool) == 0 {
	// 	return false
	// }

	bc.AddTransaction(MINING_SENDER, bc.blockchainAddress, MINING_REWARD, nil, nil)
	nonce := bc.ProofOfWork()
	previousHash := bc.LastBlock().Hash()
	bc.CreateBlock(nonce, previousHash)
	log.Println("action=mining, status=success")

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		client := &http.Client{}

		// Membuat permintaan PUT
		req, err := http.NewRequest("PUT", endpoint, nil)
		if err != nil {
			log.Printf("Error creating request to %s: %v", n, err)
			continue
		}

		// Mengirim permintaan
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error sending request to %s: %v", n, err)
			continue
		}
		defer resp.Body.Close() // Menutup body respons setelah selesai

		// Mencatat status respons
		log.Printf("Response from %s: %s", n, resp.Status)
	}
	return true
}

func (bc *Blockchain) StartMining() {
	bc.Mining()
	_ = time.AfterFunc(time.Second*MINING_TIMER_SEC, bc.StartMining)
}

func (bc *Blockchain) CalculateTotalAmount(blockchainAddress string) float32 {
	var totalAmount float32 = 0.0
	for _, b := range bc.chain {
		for _, t := range b.transactions {
			value := t.value
			if blockchainAddress == t.recipientBlockchainAddress {
				totalAmount += value
			}

			if blockchainAddress == t.senderBlockchainAddress {
				totalAmount -= value
			}
		}
	}
	return totalAmount
}

func (bc *Blockchain) ValidChain(chain []*Block) bool {
	// Log awal proses validasi
	log.Printf("Validating chain with length %d", len(chain))

	// Jika rantai kosong atau hanya satu blok, anggap valid (tergantung kebutuhan)
	if len(chain) <= 1 {
		log.Printf("Chain has 0 or 1 block, considered valid by default")
		return true
	}

	preBlock := chain[0]
	currentIndex := 1

	// Log blok awal
	log.Printf("Starting with block 0 - Hash: %x", preBlock.Hash())

	for currentIndex < len(chain) {
		b := chain[currentIndex]
		log.Printf("Checking block %d - PreviousHash: %x, Hash: %x", currentIndex, b.previousHash, b.Hash())

		// Validasi hash sebelumnya
		if b.previousHash != preBlock.Hash() {
			log.Printf("Chain invalid: Previous hash mismatch at block %d. Expected %x, got %x",
				currentIndex, preBlock.Hash(), b.previousHash)
			return false
		}
		log.Printf("Block %d: Previous hash valid", currentIndex)

		// Validasi bukti kerja
		if !bc.ValidProof(b.Nonce(), b.PreviousHash(), b.Transactions(), MINING_DIFFICULTY) {
			log.Printf("Chain invalid: Proof of work invalid at block %d. Nonce: %d, Transactions: %d",
				currentIndex, b.Nonce(), len(b.Transactions()))
			return false
		}
		log.Printf("Block %d: Proof of work valid", currentIndex)

		preBlock = b
		currentIndex += 1
	}

	log.Printf("Chain validation successful")
	return true
}

func (bc *Blockchain) ResolveConflicts() bool {
	// Spasi sebelum log fungsi dimulai
	log.Println("=====================================")
	log.Println("Starting ResolveConflicts process")

	var longestChain []*Block = nil
	maxLength := len(bc.chain)

	// Log panjang rantai lokal saat ini
	log.Printf("Current local chain length: %d", maxLength)

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/chain", n)

		// Mengirim HTTP GET request
		resp, err := http.Get(endpoint)
		if err != nil {
			log.Printf("Failed to get chain from %s: %v", n, err)
			continue
		}
		defer resp.Body.Close() // Pastikan body ditutup

		// Log status respons
		log.Printf("Response from %s: %s", n, resp.Status)

		if resp.StatusCode == 200 {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			err := decoder.Decode(&bcResp)
			if err != nil {
				log.Printf("Failed to decode chain from %s: %v", n, err)
				continue
			}

			chain := bcResp.Chain()
			chainLength := len(chain)

			// Log detail rantai yang diterima
			log.Printf("Chain received from %s - Length: %d", n, chainLength)

			if chainLength > maxLength && bc.ValidChain(chain) {
				log.Printf("Found longer valid chain from %s - New length: %d", n, chainLength)
				maxLength = chainLength
				longestChain = chain
			} else {
				log.Printf("Chain from %s not longer or invalid - Length: %d", n, chainLength)
			}
		} else {
			log.Printf("Non-200 response from %s: %s", n, resp.Status)
		}
	}

	// Menentukan hasil akhir
	if longestChain != nil {
		bc.chain = longestChain
		log.Printf("Resolve conflicts: Chain replaced with length %d", len(longestChain))
		log.Println("ResolveConflicts process completed")
		log.Println("=====================================")
		return true
	}
	log.Printf("Resolve conflicts: Chain not replaced, keeping length %d", maxLength)
	log.Println("ResolveConflicts process completed")
	log.Println("=====================================")
	return false
}

type Transaction struct {
	senderBlockchainAddress    string
	recipientBlockchainAddress string
	value                      float32
}

func NewTransaction(sender string, recipient string, value float32) *Transaction {
	return &Transaction{
		senderBlockchainAddress:    sender,
		recipientBlockchainAddress: recipient,
		value:                      value,
	}
}

func (transaction *Transaction) Print() {
	fmt.Printf("%s\n", strings.Repeat("-", 40))
	fmt.Printf(" sender_blockchain_address      %s\n", transaction.senderBlockchainAddress)
	fmt.Printf(" recipient_blockchain_address   %s\n", transaction.recipientBlockchainAddress)
	fmt.Printf(" value                          %.1f\n", transaction.value)
}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sender    string  `json:"sender_blockchain_address"`
		Recipient string  `json:"recipient_blockchain_address"`
		Value     float32 `json:"value"`
	}{
		Sender:    t.senderBlockchainAddress,
		Recipient: t.recipientBlockchainAddress,
		Value:     t.value,
	})
}

func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		Sender    *string  `json:"sender_blockchain_address"`
		Recipient *string  `json:"recipient_blockchain_address"`
		Value     *float32 `json:"value"`
	}{
		Sender:    &t.senderBlockchainAddress,
		Recipient: &t.recipientBlockchainAddress,
		Value:     &t.value,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

type TransactionRequest struct {
	SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
	SenderPublicKey            *string  `json:"sender_public_key"`
	Value                      *float32 `json:"value"`
	Signature                  *string  `json:"signature"`
}

func (tr *TransactionRequest) Validate() bool {
	if tr.SenderBlockchainAddress == nil ||
		tr.RecipientBlockchainAddress == nil ||
		tr.SenderPublicKey == nil ||
		tr.Value == nil ||
		tr.Signature == nil {
		return false
	}
	return true
}

type AmountResponse struct {
	Amount float32 `json:"amount"`
}

func (ar *AmountResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount float32 `json:"amount"`
	}{
		Amount: ar.Amount,
	})
}
