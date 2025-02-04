package balance

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/MinterTeam/minter-node-go-api"
	"github.com/MinterTeam/minter-node-go-api/responses"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
)

type Service struct {
	env                    *models.ExtenderEnvironment
	nodeApi                *minter_node_go_api.MinterNodeApi
	repository             *Repository
	addressRepository      *address.Repository
	coinRepository         *coin.Repository
	broadcastService       *broadcast.Service
	jobGetBalancesFromNode chan models.BlockAddresses
	jobUpdateBalance       chan AddressesBalancesContainer
	chAddresses            chan models.BlockAddresses
	wgBalances             sync.WaitGroup
	logger                 *logrus.Entry
}

type AddressesBalancesContainer struct {
	Addresses         []string
	Balances          []*models.Balance
	nodeApi           *minter_node_go_api.MinterNodeApi
	repository        *Repository
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	chAddresses       chan models.BlockAddresses
	broadcastService  *broadcast.Service
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, nodeApi *minter_node_go_api.MinterNodeApi,
	addressRepository *address.Repository, coinRepository *coin.Repository, broadcastService *broadcast.Service,
	logger *logrus.Entry) *Service {
	return &Service{
		env:                    env,
		repository:             repository,
		nodeApi:                nodeApi,
		addressRepository:      addressRepository,
		coinRepository:         coinRepository,
		broadcastService:       broadcastService,
		chAddresses:            make(chan models.BlockAddresses),
		jobUpdateBalance:       make(chan AddressesBalancesContainer, env.WrkUpdateBalanceCount),
		jobGetBalancesFromNode: make(chan models.BlockAddresses, env.WrkGetBalancesFromNodeCount),
		logger:                 logger,
	}
}

func (s *Service) GetAddressesChannel() chan<- models.BlockAddresses {
	return s.chAddresses
}

func (s *Service) GetBalancesFromNodeChannel() chan models.BlockAddresses {
	return s.jobGetBalancesFromNode
}

func (s *Service) GetUpdateBalancesJobChannel() chan AddressesBalancesContainer {
	return s.jobUpdateBalance
}

func (s *Service) Run() {
	for {
		addresses := <-s.chAddresses
		s.HandleAddresses(addresses)
	}
}

func (s *Service) HandleAddresses(blockAddresses models.BlockAddresses) {
	// Split addresses by chunks
	chunksCount := int(math.Ceil(float64(len(blockAddresses.Addresses)) / float64(s.env.AddrChunkSize)))
	s.wgBalances.Add(chunksCount)
	for i := 0; i < chunksCount; i++ {
		start := s.env.AddrChunkSize * i
		end := start + s.env.AddrChunkSize
		if end > len(blockAddresses.Addresses) {
			end = len(blockAddresses.Addresses)
		}
		s.GetBalancesFromNodeChannel() <- models.BlockAddresses{Height: blockAddresses.Height, Addresses: blockAddresses.Addresses[start:end]}
	}
	s.wgBalances.Wait()
}

func (s *Service) GetBalancesFromNodeWorker(jobs <-chan models.BlockAddresses, result chan<- AddressesBalancesContainer) {
	for blockAddresses := range jobs {
		addresses := make([]string, len(blockAddresses.Addresses))
		for i, adr := range blockAddresses.Addresses {
			addresses[i] = `"Mx` + adr + `"`
		}
		response, err := s.nodeApi.GetAddresses(addresses, blockAddresses.Height)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		balances, err := s.HandleBalanceResponse(response)
		if err != nil {
			s.logger.Error(err)
		} else {
			result <- AddressesBalancesContainer{Addresses: blockAddresses.Addresses, Balances: balances}
			go s.broadcastService.PublishBalances(balances)
		}
	}
}

func (s *Service) UpdateBalancesWorker(jobs <-chan AddressesBalancesContainer) {
	for container := range jobs {
		err := s.updateBalances(container.Addresses, container.Balances)
		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) HandleBalanceResponse(response *responses.BalancesResponse) ([]*models.Balance, error) {
	var balances []*models.Balance

	if len(response.Result) == 0 {
		s.logger.Warn("No data in response")
		return nil, nil
	}

	for _, item := range response.Result {
		addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(item.Address))
		if err != nil {
			s.logger.WithFields(logrus.Fields{"address": item.Address}).Error(err)
			return nil, err
		}
		for c, val := range item.Balance {
			coinId, err := s.coinRepository.FindIdBySymbol(c)
			if err != nil {
				s.logger.WithFields(logrus.Fields{"coin": c}).Error(err)
				continue
			}
			balances = append(balances, &models.Balance{
				AddressID: addressId,
				CoinID:    coinId,
				Value:     val,
			})
		}
	}
	return balances, nil
}

func (s *Service) updateBalances(addresses []string, nodeBalances []*models.Balance) error {
	defer s.wgBalances.Done()

	dbBalances, err := s.repository.FindAllByAddress(addresses)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	//If no balances in DB save all
	if dbBalances == nil {
		return s.repository.SaveAll(nodeBalances)
	}

	mapAddressBalances := makeAddressBalanceMap(dbBalances)
	var forCreate, forUpdate, forDelete []*models.Balance

	for _, nodeBalance := range nodeBalances {
		if mapAddressBalances[nodeBalance.AddressID][nodeBalance.CoinID] != nil {
			mapAddressBalances[nodeBalance.AddressID][nodeBalance.CoinID].Value = nodeBalance.Value
			forUpdate = append(forUpdate, mapAddressBalances[nodeBalance.AddressID][nodeBalance.CoinID])
			delete(mapAddressBalances[nodeBalance.AddressID], nodeBalance.CoinID)
		} else if nodeBalance.CoinID > 0 {
			forCreate = append(forCreate, nodeBalance)
			delete(mapAddressBalances[nodeBalance.AddressID], nodeBalance.CoinID)
		}
	}

	if len(forCreate) > 0 {
		err = s.repository.SaveAll(forCreate)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}

	if len(forUpdate) > 0 {
		err = s.repository.UpdateAll(forUpdate)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}

	for _, adr := range mapAddressBalances {
		for _, blc := range adr {
			forDelete = append(forDelete, blc)
		}
	}
	if len(forDelete) > 0 {
		err = s.repository.DeleteAll(forDelete)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}
	return nil
}

func makeAddressBalanceMap(balances []*models.Balance) map[uint64]map[uint64]*models.Balance {
	addrMap := make(map[uint64]map[uint64]*models.Balance)
	for _, balance := range balances {
		if val, ok := addrMap[balance.AddressID]; ok {
			val[balance.Coin.ID] = balance
		} else {
			addrMap[balance.AddressID] = make(map[uint64]*models.Balance)
			addrMap[balance.AddressID][balance.Coin.ID] = balance
		}
	}
	return addrMap
}
