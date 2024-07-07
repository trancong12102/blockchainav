package chaincode

import (
	"encoding/json"
	"errors"
	"fmt"

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

type AssetCursor struct {
	Assets   []*Asset `json:"assets"`
	Bookmark string   `json:"bookmark"`
}

var (
	ErrAssetExists = errors.New("the asset already exists")
	ErrNotFound    = errors.New("the asset does not exist")
)

// Ping is a simple function to check if the chaincode is up and running.
func (s *SmartContract) Ping(_ contractapi.TransactionContextInterface) error {
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

// ReadAssets returns paginated assets.
func (s *SmartContract) ReadAssets(
	ctx contractapi.TransactionContextInterface,
	pageSize int32,
	bookmark string,
) (*AssetCursor, error) {
	resultsIterator, metadata, err := ctx.GetStub().GetStateByRangeWithPagination("", "", pageSize, bookmark)
	if err != nil {
		return nil, fmt.Errorf("failed to read assets from world state: %w", err)
	}
	defer resultsIterator.Close()

	assets := make([]*Asset, 0)

	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read asset from iterator: %w", err)
		}

		var asset Asset

		err = json.Unmarshal(queryResponse.GetValue(), &asset)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal asset: %w", err)
		}

		assets = append(assets, &asset)
	}

	return &AssetCursor{
		Bookmark: metadata.GetBookmark(),
		Assets:   assets,
	}, nil
}
