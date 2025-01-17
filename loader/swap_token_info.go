package loader

import (
	"database/sql"
	"fmt"
	"github.com/owlto-dao/utils-go/asynccache"
	"github.com/owlto-dao/utils-go/log"
	"strings"
	"sync"
	"time"

	"github.com/owlto-dao/utils-go/alert"
)

type SwapTokenInfoManager struct {
	allTokens                  []*TokenInfo
	db                         *sql.DB
	alerter                    alert.Alerter
	mutex                      *sync.RWMutex
	chainNameTokenAddressCache asynccache.AsyncCache
	chainNameTokenNameCache    asynccache.AsyncCache
}

func NewSwapTokenInfoManager(db *sql.DB, alerter alert.Alerter) *SwapTokenInfoManager {
	chainNameTokenAddressCacheOption := asynccache.Options{
		RefreshDuration: 1 * time.Hour,
		Fetcher: func(key string) (interface{}, error) {
			s := strings.Split(key, "#")
			if len(s) != 2 {
				return nil, fmt.Errorf("invalid key: %s", key)
			}
			token, ok := GetByChainNameTokenAddrFromDb(db, s[0], s[1])
			if !ok {
				return nil, fmt.Errorf("token not found: %s", key)
			}
			return token, nil
		},
		EnableExpire:   true,
		ExpireDuration: 30 * time.Minute,
	}

	chainNameTokenNameCacheOption := asynccache.Options{
		RefreshDuration: 1 * time.Hour,
		Fetcher: func(key string) (interface{}, error) {
			s := strings.Split(key, "#")
			if len(s) != 2 {
				return nil, fmt.Errorf("invalid key: %s", key)
			}
			token, ok := GetByChainNameTokenNameFromDb(db, s[0], s[1])
			if !ok {
				return nil, fmt.Errorf("token not found: %s", key)
			}
			return token, nil
		},
		EnableExpire:   true,
		ExpireDuration: 30 * time.Minute,
	}

	return &SwapTokenInfoManager{
		db:                         db,
		alerter:                    alerter,
		mutex:                      &sync.RWMutex{},
		chainNameTokenAddressCache: asynccache.NewAsyncCache(chainNameTokenAddressCacheOption),
		chainNameTokenNameCache:    asynccache.NewAsyncCache(chainNameTokenNameCacheOption),
	}
}

func (mgr *SwapTokenInfoManager) GetByChainNameTokenAddr(chainName string, tokenAddr string) (*TokenInfo, bool) {
	key := chainName + "#" + tokenAddr
	value, err := mgr.chainNameTokenAddressCache.Get(key)
	if err != nil {
		return nil, false
	}
	if token, ok := value.(*TokenInfo); ok {
		return token, true
	} else {
		log.Errorf("swap token cache value wrong type, key %v", key)
		return nil, false
	}
}

func (mgr *SwapTokenInfoManager) GetByChainNameTokenName(chainName string, tokenName string) (*TokenInfo, bool) {
	key := chainName + "#" + tokenName
	value, err := mgr.chainNameTokenNameCache.Get(key)
	if err != nil {
		return nil, false
	}
	if token, ok := value.(*TokenInfo); ok {
		return token, true
	} else {
		log.Errorf("swap token cache value wrong type, key %v", key)
		return nil, false
	}
}

func GetByChainNameTokenAddrFromDb(db *sql.DB, chainName string, tokenAddr string) (*TokenInfo, bool) {
	var token TokenInfo
	err := db.QueryRow("SELECT token_name, chain_name, token_address, decimals, icon FROM t_swap_token_info where chain_name = ? and token_address = ?", chainName, tokenAddr).
		Scan(&token.TokenName, &token.ChainName, &token.TokenAddress, &token.Decimals, &token.Icon)
	if err != nil {
		return nil, false
	}
	return &token, true
}

func GetByChainNameTokenNameFromDb(db *sql.DB, chainName string, tokenName string) (*TokenInfo, bool) {
	var token TokenInfo
	err := db.QueryRow("SELECT token_name, chain_name, token_address, decimals, icon FROM t_swap_token_info where chain_name = ? and token_name = ?", chainName, tokenName).
		Scan(&token.TokenName, &token.ChainName, &token.TokenAddress, &token.Decimals, &token.Icon)
	if err != nil {
		return nil, false
	}
	return &token, true
}
