package loader

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/NethermindEth/starknet.go/rpc"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	solrpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/owlto-dao/utils-go/alert"
)

type Backend int32

const (
	EthereumBackend Backend = iota + 1
	StarknetBackend
	SolanaBackend
	BitcoinBackend
	ZksliteBackend
	TonBackend
	CosmosBackend
	NetworkTypeBfc
)

type ChainInfo struct {
	Id                      int64
	ChainId                 string
	RealChainId             string
	Name                    string
	AliasName               string
	Backend                 Backend
	Eip1559                 int8
	NetworkCode             int32
	Icon                    string
	BlockInterval           int32
	RpcEndPoint             string
	ExplorerUrl             string
	OfficialRpc             string
	Disabled                int8
	IsTestnet               int8
	OrderWeight             int32
	GasTokenName            string
	GasTokenDecimal         int32
	TransferContractAddress sql.NullString
	DepositContractAddress  sql.NullString
	Layer1                  sql.NullString
	Client                  interface{}
}

func (ci *ChainInfo) GetInt32ChainId() int32 {
	chainid, _ := strconv.ParseInt(ci.ChainId, 10, 32)
	return int32(chainid)
}

func (ci *ChainInfo) GetInt64ChainId() int64 {
	chainid, _ := strconv.ParseInt(ci.ChainId, 10, 64)
	return chainid
}

type ChainInfoManager struct {
	idChains      map[int64]*ChainInfo
	chainIdChains map[string]*ChainInfo
	nameChains    map[string]*ChainInfo
	netcodeChains map[int32]*ChainInfo
	allChains     []*ChainInfo

	db      *sql.DB
	alerter alert.Alerter
	mutex   *sync.RWMutex
}

func NewChainInfoManager(db *sql.DB, alerter alert.Alerter) *ChainInfoManager {
	return &ChainInfoManager{
		idChains:      make(map[int64]*ChainInfo),
		chainIdChains: make(map[string]*ChainInfo),
		nameChains:    make(map[string]*ChainInfo),
		netcodeChains: make(map[int32]*ChainInfo),
		db:            db,
		alerter:       alerter,
		mutex:         &sync.RWMutex{},
	}
}

func (mgr *ChainInfoManager) GetChainInfoIds() []int64 {
	mgr.mutex.RLock()
	ids := make([]int64, 0, len(mgr.idChains))

	for id := range mgr.idChains {
		ids = append(ids, id)
	}
	mgr.mutex.RUnlock()
	return ids
}

func (mgr *ChainInfoManager) GetChainInfoById(id int64) (*ChainInfo, bool) {
	mgr.mutex.RLock()
	chain, ok := mgr.idChains[id]
	mgr.mutex.RUnlock()
	return chain, ok
}
func (mgr *ChainInfoManager) GetChainInfoByInt32ChainId(chainId int32) (*ChainInfo, bool) {
	return mgr.GetChainInfoByChainId(strconv.FormatInt(int64(chainId), 10))
}
func (mgr *ChainInfoManager) GetChainInfoByInt64ChainId(chainId int64) (*ChainInfo, bool) {
	return mgr.GetChainInfoByChainId(strconv.FormatInt(chainId, 10))
}
func (mgr *ChainInfoManager) GetChainInfoByChainId(chainId string) (*ChainInfo, bool) {
	mgr.mutex.RLock()
	chain, ok := mgr.chainIdChains[strings.ToLower(strings.TrimSpace(chainId))]
	mgr.mutex.RUnlock()
	return chain, ok
}
func (mgr *ChainInfoManager) GetChainInfoByName(name string) (*ChainInfo, bool) {
	mgr.mutex.RLock()
	chain, ok := mgr.nameChains[strings.ToLower(strings.TrimSpace(name))]
	mgr.mutex.RUnlock()
	return chain, ok
}
func (mgr *ChainInfoManager) GetChainInfoByNetcode(netcode int32) (*ChainInfo, bool) {
	mgr.mutex.RLock()
	chain, ok := mgr.netcodeChains[netcode]
	mgr.mutex.RUnlock()
	return chain, ok
}

func (mgr *ChainInfoManager) GetAllChains() []*ChainInfo {
	return mgr.allChains
}

func (mgr *ChainInfoManager) LoadAllChains() {
	// Query the database to select only id and name fields
	rows, err := mgr.db.Query("SELECT id, chainid, real_chainid, name, alias_name, backend, eip1559, network_code, icon, block_interval, rpc_end_point, explorer_url, official_rpc, disabled, is_testnet, order_weight, gas_token_name, gas_token_decimal, transfer_contract_address, deposit_contract_address, layer1 FROM t_chain_info")

	if err != nil || rows == nil {
		mgr.alerter.AlertText("select t_chain_info error", err)
		return
	}

	defer rows.Close()

	idChains := make(map[int64]*ChainInfo)
	netcodeChains := make(map[int32]*ChainInfo)
	chainIdChains := make(map[string]*ChainInfo)
	nameChains := make(map[string]*ChainInfo)
	allChains := make([]*ChainInfo, 0)

	counter := 0

	// Iterate over the result set
	for rows.Next() {
		var chain ChainInfo

		if err := rows.Scan(&chain.Id, &chain.ChainId, &chain.RealChainId, &chain.Name, &chain.AliasName, &chain.Backend, &chain.Eip1559,
			&chain.NetworkCode, &chain.Icon, &chain.BlockInterval, &chain.RpcEndPoint, &chain.ExplorerUrl, &chain.OfficialRpc, &chain.Disabled, &chain.IsTestnet, &chain.OrderWeight,
			&chain.GasTokenName, &chain.GasTokenDecimal, &chain.TransferContractAddress, &chain.DepositContractAddress, &chain.Layer1); err != nil {
			mgr.alerter.AlertText("scan t_chain_info row error", err)
		} else {
			chain.ChainId = strings.TrimSpace(chain.ChainId)
			chain.RealChainId = strings.TrimSpace(chain.RealChainId)
			chain.Name = strings.TrimSpace(chain.Name)
			chain.AliasName = strings.TrimSpace(chain.AliasName)
			chain.Icon = strings.TrimSpace(chain.Icon)
			chain.RpcEndPoint = strings.TrimSpace(chain.RpcEndPoint)
			chain.ExplorerUrl = strings.TrimSpace(chain.ExplorerUrl)
			chain.OfficialRpc = strings.TrimSpace(chain.OfficialRpc)
			chain.GasTokenName = strings.TrimSpace(chain.GasTokenName)
			chain.TransferContractAddress.String = strings.TrimSpace(chain.TransferContractAddress.String)
			chain.DepositContractAddress.String = strings.TrimSpace(chain.DepositContractAddress.String)
			chain.Layer1.String = strings.TrimSpace(chain.Layer1.String)

			if chain.Backend == EthereumBackend {
				chain.Client, err = ethclient.Dial(chain.RpcEndPoint)
				if err != nil {
					mgr.alerter.AlertText("create evm client error", err)
					continue
				}
			} else if chain.Backend == StarknetBackend {
				erpc, err := ethrpc.Dial(chain.RpcEndPoint)
				if err != nil {
					mgr.alerter.AlertText("create starknet client error", err)
					continue
				}
				chain.Client = rpc.NewProvider(erpc)
			} else if chain.Backend == SolanaBackend {
				chain.Client = solrpc.New(chain.RpcEndPoint)
			}

			idChains[chain.Id] = &chain
			chainIdChains[strings.ToLower(chain.ChainId)] = &chain
			nameChains[strings.ToLower(chain.Name)] = &chain
			netcodeChains[chain.NetworkCode] = &chain
			allChains = append(allChains, &chain)
			counter++
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		mgr.alerter.AlertText("get next t_chain_info row error", err)
		return
	}

	mgr.mutex.Lock()
	mgr.idChains = idChains
	mgr.chainIdChains = chainIdChains
	mgr.nameChains = nameChains
	mgr.netcodeChains = netcodeChains
	mgr.allChains = allChains
	mgr.mutex.Unlock()
	log.Println("load all chain info: ", counter)

}
