package btc

import (
	"encoding/json"
	"math/big"
	"strings"

	"github.com/owlto-dao/utils-go/convert"
)

func TransferBody(receiverAddr string, amount *big.Int) ([]byte, error) {
	receiverAddr = strings.TrimSpace(receiverAddr)
	data := map[string]interface{}{
		"amount":   amount.Int64(),
		"receiver": receiverAddr,
	}
	dataStr := convert.ConvertToJsonString(data)
	m := map[string]interface{}{
		"tx_type": "Pay",
		"data":    dataStr,
	}
	return json.Marshal(m)
}
