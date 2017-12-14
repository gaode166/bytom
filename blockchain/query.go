package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/bytom/blockchain/account"
	"github.com/bytom/blockchain/query"
	log "github.com/sirupsen/logrus"

	"github.com/bytom/net/http/httpjson"
)

const (
	defGenericPageSize = 100
)

var (
	accountUTXOFmt = `
	{
		"OutputID":"%x","AssetID":"%x","Amount":"%d",
		"AccountID":"%s","ProgramIndex":"%d","Program":"%x",
		"SourceID":"%x","SourcePos":"%d","RefData":"%x","Change":"%t"
	}`
)

//
// POST /list-accounts
func (bcr *BlockchainReactor) listAccounts(ctx context.Context, query requestQuery) []byte {
	limit := query.PageSize
	if limit == 0 {
		limit = defGenericPageSize
	}

	accounts, after, last, err := bcr.accounts.ListAccounts(query.After, limit, defGenericPageSize)
	if err != nil {
		log.Errorf("listAccounts: %v", err)
		return resWrapper(nil, err)
	}

	query.After = after

	page := &page{
		Items:    httpjson.Array(accounts),
		LastPage: last,
		Next:     query}

	rawPage, err := json.Marshal(page)
	if err != nil {
		return resWrapper(nil, err)
	}

	data := []string{string(rawPage)}
	return resWrapper(data)
}

//
// POST /delete-account
func (bcr *BlockchainReactor) deleteAccount(ctx context.Context, accountInfo string) []byte {

	if err := bcr.accounts.DeleteAccount(accountInfo); err != nil {
		return resWrapper(nil, err)
	}
	return resWrapper(nil)
}

//
// POST /list-assets
func (bcr *BlockchainReactor) listAssets(ctx context.Context, query requestQuery) interface{} {

	limit := query.PageSize
	if limit == 0 {
		limit = defGenericPageSize // defGenericPageSize = 100
	}

	assets, after, last, err := bcr.assets.ListAssets(query.After, limit)
	if err != nil {
		log.Errorf("listAssets: %v", err)
		return resWrapper(nil, err)
	}

	query.After = after

	if last == false {
		last = len(assets) < limit
	}

	page := &page{
		Items:    httpjson.Array(assets),
		LastPage: last,
		Next:     query}

	rawPage, err := json.Marshal(page)
	if err != nil {
		return resWrapper(nil, err)
	}

	data := []string{string(rawPage)}
	return resWrapper(data)
}

//GetAccountUTXOs return all account unspent outputs
func (bcr *BlockchainReactor) GetAccountUTXOs() []account.UTXO {

	var (
		accountUTXO  = account.UTXO{}
		accountUTXOs = make([]account.UTXO, 0)
	)

	accountUTXOIter := bcr.wallet.DB.IteratorPrefix([]byte(account.UTXOPreFix))
	defer accountUTXOIter.Release()
	for accountUTXOIter.Next() {

		if err := json.Unmarshal(accountUTXOIter.Value(), &accountUTXO); err != nil {
			hashKey := accountUTXOIter.Key()[len(account.UTXOPreFix):]
			log.WithField("UTXO hash", string(hashKey)).Warn("get account UTXO")
			continue
		}

		accountUTXOs = append(accountUTXOs, accountUTXO)
	}

	return accountUTXOs
}

func (bcr *BlockchainReactor) listBalances(ctx context.Context, in requestQuery) interface{} {
	type assetAmount struct {
		AssetID string
		Amount  uint64
	}

	accountUTXOs := bcr.GetAccountUTXOs()
	accBalance := make(map[string]map[string]uint64)
	response := make([]string, 0)

	for _, accountUTXO := range accountUTXOs {

		assetID := fmt.Sprintf("%x", accountUTXO.AssetID)
		if _, ok := accBalance[accountUTXO.AccountID]; ok {
			if _, ok := accBalance[accountUTXO.AccountID][assetID]; ok {
				accBalance[accountUTXO.AccountID][assetID] += accountUTXO.Amount
			} else {
				accBalance[accountUTXO.AccountID][assetID] = accountUTXO.Amount
			}
		} else {
			accBalance[accountUTXO.AccountID] = map[string]uint64{assetID: accountUTXO.Amount}
		}
	}

	sortedAccount := []string{}
	for k := range accBalance {
		sortedAccount = append(sortedAccount, k)
	}
	sort.Strings(sortedAccount)

	for _, account := range sortedAccount {
		sortedAsset := []string{}
		for k := range accBalance[account] {
			sortedAsset = append(sortedAsset, k)
		}
		sort.Strings(sortedAsset)

		assetAmounts := []assetAmount{}
		for _, asset := range sortedAsset {
			assetAmounts = append(assetAmounts, assetAmount{AssetID: asset, Amount: accBalance[account][asset]})
		}

		balanceString, _ := json.Marshal(assetAmounts)
		accBalancesString := fmt.Sprintf(`{"AccountID":"%s","Balances":"%s"}`, account, balanceString)
		response = append(response, accBalancesString)
	}

	return response
}

// listTransactions is an http handler for listing transactions
//
// POST /list-transactions
func (bcr *BlockchainReactor) listTransactions(ctx context.Context, in requestQuery) []byte {

	var response = Response{Status: SUCCESS}
	annotatedTxs := make([]string, 0)
	annotatedTx := &query.AnnotatedTx{}

	txIter := bcr.wallet.DB.IteratorPrefix([]byte(query.TxPreFix))
	defer txIter.Release()

	for txIter.Next() {
		if err := json.Unmarshal(txIter.Value(), annotatedTx); err != nil {
			response.Status = FAIL
			response.Msg = err.Error()
			log.WithField("err", err).Error("failed get annotatedTx")
			break
		}
		annotatedTxs = append(annotatedTxs, string(txIter.Value()))
	}

	response.Data = annotatedTxs

	rawResponse, err := json.Marshal(response)
	if err != nil {
		return DefaultRawResponse
	}

	return rawResponse
}

// POST /list-unspent-outputs
func (bcr *BlockchainReactor) listUnspentOutputs(ctx context.Context, in requestQuery) interface{} {

	var (
		response = make([]string, 0)
		restring = ""
	)

	accountUTXOs := bcr.GetAccountUTXOs()

	for _, res := range accountUTXOs {

		restring = fmt.Sprintf(accountUTXOFmt,
			res.OutputID, res.AssetID, res.Amount,
			res.AccountID, res.ProgramIndex, res.Program,
			res.SourceID, res.SourcePos, res.RefData, res.Change)

		response = append(response, restring)
	}

	return response
}
