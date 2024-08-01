package rpc

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/metaplex/token_metadata"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/near/borsh-go"
	"github.com/owlto-dao/utils-go/loader"
	"github.com/owlto-dao/utils-go/log"
	sol "github.com/owlto-dao/utils-go/txn/solana"
	"github.com/owlto-dao/utils-go/util"
)

type MetaData struct {
	UpdateAuthority common.PublicKey
	Mint            common.PublicKey
	Name            string
	Symbol          string
	Uri             string
}
type SolanaRpc struct {
	tokenInfoMgr *loader.TokenInfoManager
	chainInfo    *loader.ChainInfo
}

func NewSolanaRpc(chainInfo *loader.ChainInfo) *SolanaRpc {
	return &SolanaRpc{
		chainInfo:    chainInfo,
		tokenInfoMgr: loader.NewTokenInfoManager(nil, nil),
	}
}

func (w *SolanaRpc) GetClient() *rpc.Client {
	return w.chainInfo.Client.(*rpc.Client)
}

func (w *SolanaRpc) GetAccountInfo(ctx context.Context, owner solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	rsp, err := w.GetClient().GetAccountInfoWithOpts(
		ctx,
		owner,
		&rpc.GetAccountInfoOpts{
			Commitment: rpc.CommitmentConfirmed,
			DataSlice:  nil,
		},
	)

	if err != nil {
		return nil, err
	} else {
		return rsp, nil
	}
}

func (w *SolanaRpc) GetStrAccountInfo(ctx context.Context, ownerAddr string) (*rpc.GetAccountInfoResult, error) {
	ownerAddr = strings.TrimSpace(ownerAddr)

	ownerpk, err := solana.PublicKeyFromBase58(ownerAddr)
	if err != nil {
		return nil, err
	}

	return w.GetAccountInfo(ctx, ownerpk)

}

func getExtensionData(extensionType uint16, tlvData []byte) []byte {
	extensionTypeIndex := 0
	for extensionTypeIndex+4 <= len(tlvData) {
		entryType := binary.LittleEndian.Uint16(tlvData[extensionTypeIndex : extensionTypeIndex+2])
		entryLength := binary.LittleEndian.Uint16(tlvData[extensionTypeIndex+2 : extensionTypeIndex+4])
		typeIndex := extensionTypeIndex + 4
		if entryType == extensionType && typeIndex+int(entryLength) <= len(tlvData) {
			return tlvData[typeIndex : typeIndex+int(entryLength)]
		}
		extensionTypeIndex = typeIndex + int(entryLength)
	}
	return nil
}

func (w *SolanaRpc) GetTokenInfo(ctx context.Context, tokenAddr string) (loader.TokenInfo, error) {
	if util.IsHexStringZero(tokenAddr) || tokenAddr == "11111111111111111111111111111111" {
		return loader.TokenInfo{
			TokenName:    "SOL",
			ChainName:    w.chainInfo.Name,
			TokenAddress: tokenAddr,
			Decimals:     9,
			FullName:     "Solana",
			TotalSupply:  0,
			Url:          "https://solana.com",
		}, nil
	}
	tokenInfo, ok := w.tokenInfoMgr.GetByChainNameTokenAddr(w.chainInfo.Name, tokenAddr)
	if ok {
		return *tokenInfo, nil
	}

	mintpk, err := solana.PublicKeyFromBase58(tokenAddr)
	if err != nil {
		return loader.TokenInfo{}, err
	}

	metapk, _, err := solana.FindTokenMetadataAddress(mintpk)
	if err != nil {
		return loader.TokenInfo{}, err
	}

	symbol := "UNKNOWN"
	fullName := "UNKNOWN"
	uri := ""
	rsp, err := w.GetAccountInfo(
		ctx,
		metapk,
	)
	if err == nil {
		meta, err := token_metadata.MetadataDeserialize(rsp.GetBinary())
		if err != nil {
			return loader.TokenInfo{}, err
		}
		symbol = meta.Data.Symbol
		fullName = meta.Data.Name
		uri = meta.Data.Uri
	} else if err != rpc.ErrNotFound {
		return loader.TokenInfo{}, err
	}

	rsp, err = w.GetAccountInfo(
		ctx,
		mintpk,
	)
	if err != nil {
		return loader.TokenInfo{}, err
	}
	var mintAccount token.Mint
	data := rsp.GetBinary()
	decoder := bin.NewBorshDecoder(data)
	err = mintAccount.UnmarshalWithDecoder(decoder)
	if err != nil {
		return loader.TokenInfo{}, err
	}

	if len(data) > 166 {
		var metadata MetaData
		err := borsh.Deserialize(&metadata, getExtensionData(19, []byte(data[166:])))
		if err == nil {
			symbol = metadata.Symbol
			fullName = metadata.Name
			uri = metadata.Uri
		}
	}

	token := loader.TokenInfo{
		TokenName:    symbol,
		ChainName:    w.chainInfo.Name,
		TokenAddress: tokenAddr,
		Decimals:     int32(mintAccount.Decimals),
		FullName:     fullName,
		TotalSupply:  mintAccount.Supply,
		Url:          uri,
	}
	w.tokenInfoMgr.AddTokenInfo(token)
	return token, nil

}

func (w *SolanaRpc) GetSplAccount(ctx context.Context, ownerAddr string, tokenAddr string) (*token.Account, error) {
	ownerAddr = strings.TrimSpace(ownerAddr)
	tokenAddr = strings.TrimSpace(tokenAddr)

	ownerpk, err := solana.PublicKeyFromBase58(ownerAddr)
	if err != nil {
		return nil, err
	}
	mintpk, err := solana.PublicKeyFromBase58(tokenAddr)
	if err != nil {
		return nil, err
	}

	ownerAta, err := sol.GetAtaFromPk(ownerpk, mintpk)
	if err != nil {
		return nil, err
	}

	rsp, err := w.GetAccountInfo(
		ctx,
		ownerAta,
	)
	if err != nil {
		return nil, err
	}

	var tokenAccount token.Account
	decoder := bin.NewBorshDecoder(rsp.GetBinary())
	err = tokenAccount.UnmarshalWithDecoder(decoder)
	if err != nil {
		return nil, err
	} else {
		return &tokenAccount, nil
	}
}

func (w *SolanaRpc) GetBalance(ctx context.Context, ownerAddr string, tokenAddr string) (*big.Int, error) {
	ownerAddr = strings.TrimSpace(ownerAddr)
	tokenAddr = strings.TrimSpace(tokenAddr)

	if util.IsHexStringZero(tokenAddr) || tokenAddr == "11111111111111111111111111111111" {
		accountInfo, err := w.GetStrAccountInfo(ctx, ownerAddr)
		if err != nil {
			if err == rpc.ErrNotFound {
				return big.NewInt(0), nil
			}
			return nil, err
		}
		return big.NewInt(int64(accountInfo.Value.Lamports)), nil
	} else {
		sqlAccount, err := w.GetSplAccount(ctx, ownerAddr, tokenAddr)
		if err != nil {
			if err == rpc.ErrNotFound {
				return big.NewInt(0), nil
			}
			return nil, err
		}
		return big.NewInt(int64(sqlAccount.Amount)), nil
	}
}

func (w *SolanaRpc) GetAllowance(ctx context.Context, ownerAddr string, tokenAddr string, spenderAddr string) (*big.Int, error) {
	sqlAccount, err := w.GetSplAccount(ctx, ownerAddr, tokenAddr)
	if err != nil {
		if err == rpc.ErrNotFound {
			return big.NewInt(0), nil
		}
		return nil, err
	} else {
		return big.NewInt(int64(sqlAccount.DelegatedAmount)), nil
	}
}

func (w *SolanaRpc) IsTxSuccess(ctx context.Context, hash string) (bool, int64, error) {
	sig, err := solana.SignatureFromBase58(hash)
	if err != nil {
		return false, 0, err
	}

	receipt, err := w.GetClient().GetTransaction(ctx, sig, nil)
	if err != nil {
		return false, 0, err
	}
	if receipt == nil {
		return false, 0, fmt.Errorf("get receipt failed")
	}
	return receipt.Meta.Err == nil, int64(receipt.Slot), nil
}

func (w *SolanaRpc) Client() interface{} {
	return w.chainInfo.Client
}

func (w *SolanaRpc) Backend() int32 {
	return 3
}

func (w *SolanaRpc) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	blockNumber, err := w.GetClient().GetSlot(
		context.TODO(),
		rpc.CommitmentFinalized,
	)

	if err != nil {
		log.Errorf("%v get latest block number error %v", w.chainInfo.Name, err)
		return 0, err
	}
	return int64(blockNumber), nil

}
