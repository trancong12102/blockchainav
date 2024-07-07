package chaincode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing an Asset.
type SmartContract struct {
	contractapi.Contract
}

type Asset struct {
	CID      string `json:"cid"`
	Features string `json:"features"`
	ID       string `json:"id"`
	Type     string `json:"type"`
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

// SeedLedger creates 100 assets in the ledger.
func (s *SmartContract) SeedLedger(ctx contractapi.TransactionContextInterface) error {
	types := []string{
		"PDF",
		"PE",
	}

	for i := range 100 {
		asset := Asset{
			ID:       fmt.Sprintf("ASSET_%d", i),
			CID:      fmt.Sprintf("CID_%d", i),
			Type:     types[i%2],
			Features: "[]",
		}

		err := s.CreateAsset(ctx, asset.ID, asset.CID, asset.Type, asset.Features)
		if err != nil {
			return fmt.Errorf("failed to create asset: %w", err)
		}
	}

	return nil
}

// DeleteSeededAssets deletes all seeded assets from the ledger.
func (s *SmartContract) DeleteSeededAssets(ctx contractapi.TransactionContextInterface) error {
	for i := range 100 {
		if err := ctx.GetStub().DelState(fmt.Sprintf("ASSET_%d", i)); err != nil {
			return fmt.Errorf("failed to delete asset: %w", err)
		}
	}

	return nil
}

// CreateAsset issues a new asset to the world state with given details.
func (s *SmartContract) CreateAsset(
	ctx contractapi.TransactionContextInterface,
	id string,
	cid string,
	assetType string,
	features string,
) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}

	if exists {
		return ErrAssetExists
	}

	asset := Asset{
		ID:       id,
		CID:      cid,
		Type:     assetType,
		Features: features,
	}

	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return fmt.Errorf("failed to marshal asset: %w", err)
	}

	if err := ctx.GetStub().PutState(id, assetJSON); err != nil {
		return fmt.Errorf("failed to put asset in world state: %w", err)
	}

	return nil
}

// ReadAsset returns the asset stored in the world state with given id.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %w", err)
	}

	if assetJSON == nil {
		return nil, ErrNotFound
	}

	var asset Asset

	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal asset: %w", err)
	}

	return &asset, nil
}

// AssetExists returns true when asset with given ID exists in world state.
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %w", err)
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
		return nil, fmt.Errorf("failed to get query result: %w", err)
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
			return nil, fmt.Errorf("failed to read asset from iterator: %w", err)
		}

		var asset Asset

		err = json.Unmarshal(queryResult.GetValue(), &asset)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset: %w", err)
		}

		assets = append(assets, &asset)
	}

	return assets, nil
}
