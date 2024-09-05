package helper

import (
	"context"
	"sync"
	"time"

	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"

	"github.com/pkg/errors"

	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"

	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

var kzg4844Context *gokzg4844.Context

// From ethereum/rpc:

// LoggingRoundTrip writes requests and responses to the test log.
type LogF interface {
	Logf(format string, values ...interface{})
}

type LoggingRoundTrip struct {
	Logger LogF
	ID     string
	Inner  http.RoundTripper
}

const MAX_LOG_BYTES = 1024 * 4

func (rt *LoggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	reqLogBytes := bytes.TrimSpace(reqBytes[:])
	if len(reqLogBytes) > MAX_LOG_BYTES {
		rt.Logger.Logf(">> (%s) %s... (Log trimmed)", rt.ID, reqLogBytes[:MAX_LOG_BYTES])
	} else {
		rt.Logger.Logf(">> (%s) %s", rt.ID, reqLogBytes)
	}
	reqCopy := *req
	reqCopy.Body = io.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.Inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = io.NopCloser(bytes.NewReader(respBytes))
	respLogBytes := bytes.TrimSpace(respBytes[:])
	if len(respLogBytes) > MAX_LOG_BYTES {
		rt.Logger.Logf("<< (%s) %s... (Log trimmed)", rt.ID, respLogBytes[:MAX_LOG_BYTES])
	} else {
		rt.Logger.Logf("<< (%s) %s", rt.ID, respLogBytes)
	}
	return &respCopy, nil
}

type InvalidPayloadBlockField string

const (
	InvalidParentHash             = "ParentHash"
	InvalidStateRoot              = "StateRoot"
	InvalidReceiptsRoot           = "ReceiptsRoot"
	InvalidNumber                 = "Number"
	InvalidGasLimit               = "GasLimit"
	InvalidGasUsed                = "GasUsed"
	InvalidTimestamp              = "Timestamp"
	InvalidPrevRandao             = "PrevRandao"
	InvalidOmmers                 = "Ommers"
	InvalidWithdrawals            = "Withdrawals"
	InvalidBlobGasUsed            = "BlobGasUsed"
	InvalidBlobCountGasUsed       = "Blob Count on BlobGasUsed"
	InvalidExcessBlobGas          = "ExcessBlobGas"
	InvalidParentBeaconBlockRoot  = "ParentBeaconBlockRoot"
	InvalidVersionedHashes        = "VersionedHashes"
	InvalidVersionedHashesVersion = "VersionedHashes Version"
	IncompleteVersionedHashes     = "Incomplete VersionedHashes"
	ExtraVersionedHashes          = "Extra VersionedHashes"
	RemoveTransaction             = "Incomplete Transactions"
	InvalidTransactionSignature   = "Transaction Signature"
	InvalidTransactionNonce       = "Transaction Nonce"
	InvalidTransactionGas         = "Transaction Gas"
	InvalidTransactionGasPrice    = "Transaction GasPrice"
	InvalidTransactionGasTipPrice = "Transaction GasTipCapPrice"
	InvalidTransactionValue       = "Transaction Value"
	InvalidTransactionChainID     = "Transaction ChainID"
)

func TransactionInPayload(payload *typ.ExecutableData, tx typ.Transaction) bool {
	for _, bytesTx := range payload.Transactions {
		var currentTx types.Transaction
		if err := currentTx.UnmarshalBinary(bytesTx); err == nil {
			if currentTx.Hash() == tx.Hash() {
				return true
			}
		}
	}
	return false
}

// Use client specific rpc methods to debug a transaction that includes the PREVRANDAO opcode
func DebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, clientType string, tx typ.Transaction, expectedPrevRandao *common.Hash) error {
	switch clientType {
	case "go-ethereum":
		return gethDebugPrevRandaoTransaction(ctx, c, tx, expectedPrevRandao)
	case "nethermind-old":
		return nethermindDebugPrevRandaoTransaction(ctx, c, tx, expectedPrevRandao)
	}
	fmt.Printf("debug_traceTransaction, no method to test client type %v", clientType)
	return nil
}

func gethDebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, tx typ.Transaction, expectedPrevRandao *common.Hash) error {
	type StructLogRes struct {
		Pc      uint64             `json:"pc"`
		Op      string             `json:"op"`
		Gas     uint64             `json:"gas"`
		GasCost uint64             `json:"gasCost"`
		Depth   int                `json:"depth"`
		Error   string             `json:"error,omitempty"`
		Stack   *[]string          `json:"stack,omitempty"`
		Memory  *[]string          `json:"memory,omitempty"`
		Storage *map[string]string `json:"storage,omitempty"`
	}

	type ExecutionResult struct {
		Gas         uint64         `json:"gas"`
		Failed      bool           `json:"failed"`
		ReturnValue string         `json:"returnValue"`
		StructLogs  []StructLogRes `json:"structLogs"`
	}

	var er *ExecutionResult
	if err := c.CallContext(ctx, &er, "debug_traceTransaction", tx.Hash()); err != nil {
		return err
	}
	if er == nil {
		return errors.New("debug_traceTransaction returned empty result")
	}
	prevRandaoFound := false
	for i, l := range er.StructLogs {
		if l.Op == "DIFFICULTY" || l.Op == "PREVRANDAO" {
			if i+1 >= len(er.StructLogs) {
				return errors.New("no information after PREVRANDAO operation")
			}
			prevRandaoFound = true
			stack := *(er.StructLogs[i+1].Stack)
			if len(stack) < 1 {
				return fmt.Errorf("invalid stack after PREVRANDAO operation: %v", l.Stack)
			}
			stackHash := common.HexToHash(stack[0])
			if stackHash != *expectedPrevRandao {
				return fmt.Errorf("invalid stack after PREVRANDAO operation, %v != %v", stackHash, expectedPrevRandao)
			}
		}
	}
	if !prevRandaoFound {
		return errors.New("PREVRANDAO opcode not found")
	}
	return nil
}

func nethermindDebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, tx typ.Transaction, expectedPrevRandao *common.Hash) error {
	var er *interface{}
	if err := c.CallContext(ctx, &er, "trace_transaction", tx.Hash()); err != nil {
		return err
	}
	return nil
}

func bytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

func LoadChain(path string) types.Blocks {
	fh, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer fh.Close()
	var reader io.Reader = fh
	stream := rlp.NewStream(reader, 0)

	blocks := make(types.Blocks, 0)
	for {
		var b types.Block
		if err := stream.Decode(&b); err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		blocks = append(blocks, &b)
	}
	return blocks
}

func LoadGenesis(path string, genesis client.Genesis) client.Genesis {
	reader, err := os.Open(path)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}

	//var genesis NethermindChainSpec
	err = genesis.UnmarshalJSON(contents)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(contents, &genesis); err != nil {
		panic(fmt.Errorf("can't parse genesis JSON: %v", err))
	}
	return genesis
}

func LoadGenesisTest(path string) string {
	reader, err := os.Open(path)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	return string(contents)
}

func GenesisStartOptionBasedOnClient(genesis client.Genesis, clientName string) (hivesim.StartOption, error) {
	out, err := genesis.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	// We don't need mappers
	switch clientName {
	case "erigon":
		return hivesim.WithDynamicFile("/genesis.json", bytesSource(out)), nil
	case "nethermind-old":
		return hivesim.WithDynamicFile("/chainspec.json", bytesSource(out)), nil
	}
	return hivesim.WithDynamicFile("/genesis.json", bytesSource(out)), nil
}

func CalculateTotalDifficulty(genesis client.Genesis, chain types.Blocks, lastBlock uint64) *big.Int {
	result := new(big.Int).Set(genesis.Difficulty())
	for _, b := range chain {
		result.Add(result, b.Difficulty())
		if lastBlock != 0 && lastBlock == b.NumberU64() {
			break
		}
	}
	return result
}

// TTD is the value specified in the test.Spec + Genesis.Difficulty
func CalculateRealTTD(g client.Genesis, ttdValue int64) int64 {
	return g.Config().TerminalTotalDifficulty.Int64() + ttdValue
}

var (
	// Time between checks of a client reaching Terminal Total Difficulty
	TTDCheckPeriod = time.Second * 1
)

// TTD Check Methods
func CheckTTD(ec client.EngineClient, ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, globals.RPCTimeout)
	defer cancel()
	td, err := ec.GetTotalDifficulty(ctx)
	if err == nil {
		return td.Cmp(ec.TerminalTotalDifficulty()) >= 0, nil
	}
	return false, err
}

type WaitTTDResponse struct {
	ec  client.EngineClient
	err error
}

// Wait until the TTD is reached by a single client.
func WaitForTTD(ec client.EngineClient, wg *sync.WaitGroup, done chan<- WaitTTDResponse, ctx context.Context) {
	defer wg.Done()
	for {
		select {
		case <-time.After(TTDCheckPeriod):
			ttdReached, err := CheckTTD(ec, ctx)
			if err == nil && ttdReached {
				select {
				case done <- WaitTTDResponse{
					ec:  ec,
					err: err,
				}:
				case <-ctx.Done():
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// Wait until the TTD is reached by a single client with a timeout.
// Returns true if the TTD has been reached, false when timeout occurred.
func WaitForTTDWithTimeout(ec client.EngineClient, ctx context.Context) error {
	for {
		select {
		case <-time.After(TTDCheckPeriod):
			ttdReached, err := CheckTTD(ec, ctx)
			if err != nil {
				return err
			}
			if ttdReached {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timeout reached")
		}
	}
}

// Wait until the TTD is reached by any of the engine clients
func WaitAnyClientForTTD(ecs []client.EngineClient, testCtx context.Context) (client.EngineClient, error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	done := make(chan WaitTTDResponse)
	cancel := make(chan interface{})
	defer close(cancel)
	for _, ec := range ecs {
		wg.Add(1)
		ctx, cancel := context.WithCancel(testCtx)
		defer cancel()
		go WaitForTTD(ec, &wg, done, ctx)
	}
	select {
	case r := <-done:
		return r.ec, r.err
	case <-testCtx.Done():
		return nil, testCtx.Err()
	}
}
func SendNextTransactionWithAccount(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator, sender *globals.TestAccount) (typ.Transaction, error) {
	nonce, err := node.GetNextAccountNonce(testCtx, sender.GetAddress(), nil)
	if err != nil {
		return nil, err
	}
	tx, err := txCreator.MakeTransaction(sender, nonce, uint64(time.Now().Unix()))
	if err != nil {
		return nil, err
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil {
			return tx, nil
		} else if SentTxAlreadyKnown(err) {
			return tx, nil
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, testCtx.Err()
		}
	}
}
