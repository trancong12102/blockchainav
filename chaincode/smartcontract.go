package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing an Asset.
type SmartContract struct {
	contractapi.Contract
}

// ENUM(PDF, PE).
type AssetType string

type Asset struct {
	CID      string    `json:"cid"`
	Features string    `json:"features"`
	ID       string    `json:"id"`
	Type     AssetType `json:"type"`
}

// PaginatedQueryResult structure used for returning paginated query results and metadata.
type PaginatedQueryResult struct {
	Records             []*Asset `json:"records"`
	FetchedRecordsCount int32    `json:"fetchedRecordsCount"`
	Bookmark            string   `json:"bookmark"`
}

var (
	ErrAssetExists = errors.New("the asset already exists")
	ErrNotFound    = errors.New("the asset does not exist")
)

// Ping is a simple function to check if the chaincode is up and running.
func (s *SmartContract) Ping(_ contractapi.TransactionContextInterface) error {
	return nil
}

// CreateAsset issues a new asset to the ledger state with given details.
func (s *SmartContract) CreateAsset(
	ctx contractapi.TransactionContextInterface,
	assetCID string,
	assetID string,
	assetType string,
	features string,
) error {
	exists, err := s.AssetExists(ctx, assetCID)
	if err != nil {
		return err
	}

	if exists {
		return ErrAssetExists
	}

	assetTypeEnum, err := ParseAssetType(assetType)
	if err != nil {
		return fmt.Errorf("parse asset type: %w", err)
	}

	asset := Asset{
		ID:       assetID,
		CID:      assetCID,
		Type:     assetTypeEnum,
		Features: features,
	}

	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return fmt.Errorf("marshal asset: %w", err)
	}

	if err := ctx.GetStub().PutState(assetCID, assetJSON); err != nil {
		return fmt.Errorf("put asset in ledger state: %w", err)
	}

	return nil
}

// GetAsset returns the asset stored in the ledger state with given id.
func (s *SmartContract) GetAsset(ctx contractapi.TransactionContextInterface, cid string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(cid)
	if err != nil {
		return nil, fmt.Errorf("read from ledger state: %w", err)
	}

	if assetJSON == nil {
		return nil, ErrNotFound
	}

	var asset Asset

	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, fmt.Errorf("unmarshal asset: %w", err)
	}

	return &asset, nil
}

// AssetExists returns true when asset with given ID exists in ledger state.
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, cid string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(cid)
	if err != nil {
		return false, fmt.Errorf("read from ledger state: %w", err)
	}

	return assetJSON != nil, nil
}

// QueryAssets uses a query string, page size and a bookmark to perform a query
// for assets. Query string matching state database syntax is passed in and executed as is.
// The number of fetched records would be equal to or lesser than the specified page size.
// Supports ad hoc queries that can be defined at runtime by the client.
// Only available on state databases that support rich query (e.g. CouchDB)
// Paginated queries are only valid for read only transactions.
// Example: Pagination with Ad hoc Rich Query.
func (s *SmartContract) QueryAssets(
	ctx contractapi.TransactionContextInterface,
	queryString string,
	pageSize int,
	bookmark string,
) (*PaginatedQueryResult, error) {
	return getQueryResultForQueryStringWithPagination(ctx, queryString, int32(pageSize), bookmark)
}

// getQueryResultForQueryStringWithPagination executes the passed in query string with
// pagination info. The result set is built and returned as a byte array containing the JSON results.
func getQueryResultForQueryStringWithPagination(
	ctx contractapi.TransactionContextInterface,
	queryString string,
	pageSize int32,
	bookmark string,
) (*PaginatedQueryResult, error) {
	resultsIterator, responseMetadata, err := ctx.GetStub().GetQueryResultWithPagination(queryString, pageSize, bookmark)
	if err != nil {
		return nil, fmt.Errorf("get query result: %w", err)
	}
	defer resultsIterator.Close()

	assets, err := constructQueryResponseFromIterator(resultsIterator)
	if err != nil {
		return nil, err
	}

	return &PaginatedQueryResult{
		Records:             assets,
		FetchedRecordsCount: responseMetadata.GetFetchedRecordsCount(),
		Bookmark:            responseMetadata.GetBookmark(),
	}, nil
}

// constructQueryResponseFromIterator constructs a slice of assets from the resultsIterator.
func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) ([]*Asset, error) {
	assets := make([]*Asset, 0)

	for resultsIterator.HasNext() {
		queryResult, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("read asset from iterator: %w", err)
		}

		var asset Asset

		err = json.Unmarshal(queryResult.GetValue(), &asset)
		if err != nil {
			return nil, fmt.Errorf("unmarshal asset: %w", err)
		}

		assets = append(assets, &asset)
	}

	return assets, nil
}

func startChaincode() error {
	chaincode, err := contractapi.NewChaincode(new(SmartContract))
	if err != nil {
		return fmt.Errorf("creating blockchain-av chaincode: %w", err)
	}

	if err := chaincode.Start(); err != nil {
		return fmt.Errorf("starting blockchain-av chaincode: %w", err)
	}

	return nil
}

func main() {
	err := startChaincode()
	if err != nil {
		slog.Error("start chaincode", slog.Any("error", err))
		os.Exit(1)
	}
}
