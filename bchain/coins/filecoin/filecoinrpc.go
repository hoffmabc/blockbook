package filecoin

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/json"
	"github.com/hoffmabc/blockbook/bchain/coins/eth"

	//"net/rpc"

	"github.com/juju/errors"

	"github.com/golang/glog"

	"github.com/filecoin-project/lotus/api"
)

// Configuration represents json config file
type Configuration struct {
	CoinName                    string `json:"coin_name"`
	CoinShortcut                string `json:"coin_shortcut"`
	RPCURL                      string `json:"rpc_url"`
	RPCTimeout                  int    `json:"rpc_timeout"`
	BlockAddressesToKeep        int    `json:"block_addresses_to_keep"`
	MempoolTxTimeoutHours       int    `json:"mempoolTxTimeoutHours"`
	QueryBackendOnMempoolResync bool   `json:"queryBackendOnMempoolResync"`
}

// FloRPC is an interface to JSON-RPC bitcoind service.
type FilecoinRPC struct {
	//*btc.BitcoinRPC
	fullNode    api.FullNode
	Parser      *FilecoinParser
	ChainConfig *Configuration
	Mempool     *bchain.MempoolFilecoinType
}

// NewFilecoinRPC returns new FilecoinRPC instance.
func NewFilecoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	//var err error
	var c Configuration
	err = json.Unmarshal(config, &c)

	//b, err := btc.NewBitcoinRPC(config, pushHandler)
	//if err != nil {
	//	return nil, err
	//}

	s := &FilecoinRPC{
		//b.(*btc.BitcoinRPC),
		fullNode: client.NewFullNodeRPC("http://"+c.RPCURL, nil),
		config:   c,
	}

	// always create parser
	s.Parser = NewFilecoinParser(s.ChainConfig)

	return s, nil
}

// Initialize initializes FilecoinRPC instance.
func (f *FilecoinRPC) Initialize() error {

	//f.Network = "testnet"
	//f.Testnet = true

	// parameters for getInfo request
	//if params.Net == MainnetMagic {
	//	f.Testnet = false
	//	f.Network = "livenet"
	//} else {
	//	f.Testnet = true
	//	f.Network = "testnet"
	//}

	//glog.Info("rpc: block chain ", params.Name)

	return nil
}

// GetBlock returns block with given hash.
func (f *FilecoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {

	// ChainGetBlock
	//

	var err error
	if hash == "" {
		hash, err = f.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
	}
	if !f.ParseBlocks {
		return f.GetBlockFull(hash)
	}
	// optimization
	if height > 0 {
		return f.GetBlockWithoutHeader(hash, height)
	}
	header, err := f.GetBlockHeader(hash)
	if err != nil {
		return nil, err
	}
	data, err := f.GetBlockRaw(hash)
	if err != nil {
		return nil, err
	}
	block, err := f.Parser.ParseBlock(data)
	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	block.BlockHeader = *header
	return block, nil
}

// GetBlockFull returns block with given hash
func (f *FilecoinRPC) GetBlockFull(hash string) (*bchain.Block, error) {
	glog.V(1).Info("rpc: getblock (verbosity=2) ", hash)

	res := btc.ResGetBlockFull{}
	req := btc.CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = 2
	err := f.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if btc.IsErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}

	for i := range res.Result.Txs {
		tx := &res.Result.Txs[i]
		for j := range tx.Vout {
			vout := &tx.Vout[j]
			// convert vout.JsonValue to big.Int and clear it, it is only temporary value used for unmarshal
			vout.ValueSat, err = f.Parser.AmountToBigInt(vout.JsonValue)
			if err != nil {
				return nil, err
			}
			vout.JsonValue = ""
		}
	}

	return &res.Result, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (f *FilecoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return f.GetTransaction(txid)
}

// CreateMempool creates mempool if not already created, however does not initialize it
func (b *FilecoinRPC) CreateMempool(chain bchain.BlockChain) (bchain.Mempool, error) {
	if b.Mempool == nil {
		b.Mempool = bchain.NewMempoolFilecoinType(chain, b.ChainConfig.MempoolTxTimeoutHours, b.ChainConfig.QueryBackendOnMempoolResync)
		glog.Info("mempool created, MempoolTxTimeoutHours=", b.ChainConfig.MempoolTxTimeoutHours, ", QueryBackendOnMempoolResync=", b.ChainConfig.QueryBackendOnMempoolResync)
	}
	return b.Mempool, nil
}