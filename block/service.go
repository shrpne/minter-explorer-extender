package block

import (
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api/responses"
	"strconv"
	"time"
)

type Service struct {
	blockRepository     *Repository
	validatorRepository *validator.Repository
	blockCache          *models.Block //Contain previous block model
}

func NewBlockService(blockRepository *Repository, validatorRepository *validator.Repository) *Service {
	return &Service{
		blockRepository:     blockRepository,
		validatorRepository: validatorRepository,
	}
}

func (s *Service) SetBlockCache(b *models.Block) {
	s.blockCache = b
}

//Handle response and save block to DB
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) error {

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)
	totalTx, err := strconv.ParseUint(response.Result.TotalTx, 10, 64)
	helpers.HandleError(err)
	numTx, err := strconv.ParseUint(response.Result.TxCount, 10, 32)
	helpers.HandleError(err)
	size, err := strconv.ParseUint(response.Result.Size, 10, 64)
	helpers.HandleError(err)

	proposerPk := []rune(response.Result.Proposer)
	proposerId, err := s.validatorRepository.FindIdOrCreateByPk(string(proposerPk[2:]))

	block := &models.Block{
		ID:                  height,
		TotalTxs:            totalTx,
		NumTxs:              uint32(numTx),
		Size:                size,
		BlockTime:           s.getBlockTime(response.Result.Time),
		CreatedAt:           response.Result.Time,
		BlockReward:         response.Result.BlockReward,
		ProposerValidatorID: proposerId,
		Hash:                response.Result.Hash,
	}

	s.blockCache = block

	return s.blockRepository.Save(block)
}

func (s *Service) getBlockTime(blockTime time.Time) uint64 {
	if s.blockCache == nil {
		return 1000000000 //ns, 1 second for the firs block
	}
	result := blockTime.Sub(s.blockCache.CreatedAt)
	return uint64(result.Nanoseconds())
}