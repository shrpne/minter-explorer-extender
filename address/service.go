package address

import (
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api/responses"
)

type Service struct {
	repository         *Repository
	chBalanceAddresses chan<- models.BlockAddresses
}

func NewService(repository *Repository, chBalanceAddresses chan<- models.BlockAddresses) *Service {
	return &Service{
		repository:         repository,
		chBalanceAddresses: chBalanceAddresses,
	}
}

// Find all addresses in block response and save it
func (s *Service) HandleTransactionsFromBlockResponse(height uint64, transactions []responses.Transaction) error {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		if tx.Data == nil {
			return errors.New("empty transaction data")
		}
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}
		if tx.Type == models.TxTypeSend {
			var txData models.SendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return err
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return err
			}
			mapAddresses[helpers.RemovePrefix(txData.To)] = struct{}{}
		}
		if tx.Type == models.TxTypeMultiSend {
			var txData models.MultiSendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return err
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return err
			}
			for _, receiver := range txData.List {
				mapAddresses[helpers.RemovePrefix(receiver.To)] = struct{}{}
			}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)
	err := s.repository.SaveAllIfNotExist(addresses)
	s.chBalanceAddresses <- models.BlockAddresses{Height: height, Addresses: addresses}
	return err
}

func (s *Service) HandleEventsResponse(blockHeight uint64, response *responses.EventsResponse) error {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, event := range response.Result.Events {
		mapAddresses[helpers.RemovePrefix(event.Value.Address)] = struct{}{}
	}
	addresses := addressesMapToSlice(mapAddresses)
	err := s.repository.SaveAllIfNotExist(addresses)
	s.chBalanceAddresses <- models.BlockAddresses{Height: blockHeight, Addresses: addresses}
	return err
}

func addressesMapToSlice(mapAddresses map[string]struct{}) []string {
	addresses := make([]string, len(mapAddresses))
	i := 0
	for a := range mapAddresses {
		addresses[i] = a
		i++
	}
	return addresses
}
