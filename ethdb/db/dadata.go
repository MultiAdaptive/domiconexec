package db

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/kzg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"gorm.io/gorm"
	"time"
)

const layout = "2006-01-02T15:04:05Z07:00"

// 创建commitment表格模型
type DA struct {
	gorm.Model
	Nonce           int64
	Sender          string
	Index           int64
	Length          int64
	TxHash          string `gorm:"unique;column:tx_hash"`
	Commitment      string
	CommitmentHash  string
	Data            string
	DAsKey          string
	SignData        []string
	SignAddr        []string
	ParentStateHash string //parent Commit Data Hash
	StateHash       string //latest commit Data hash
	BlockNum        int64
	ReceiveAt       string
}

func AddCommitment(tx *gorm.DB, da *types.DA, parentHash common.Hash) error {
	currentParentHash := parentHash
	dataCollect := make([]byte, 0)
	dataCollect = append(dataCollect, da.Commitment.X.Marshal()...)
	dataCollect = append(dataCollect, da.Commitment.Y.Marshal()...)
	dataCollect = append(dataCollect, da.Sender.Bytes()...)
	dataCollect = append(dataCollect, currentParentHash.Bytes()...)
	stateHash := common.BytesToHash(dataCollect)

	sigDatStr := make([]string, len(da.SignData))
	for i,data :=range da.SignData {
		sigDatStr[i] = common.Bytes2Hex(data)
	}
	addrStr := make([]string,len(da.SignerAddr))
	for i,addr := range da.SignerAddr{
		addrStr[i] = addr.Hex()
	}

	wd := DA{
		Sender:          da.Sender.Hex(),
		Index:           int64(da.Index),
		Length:          int64(da.Length),
		TxHash:          da.TxHash.Hex(),
		Commitment:      common.Bytes2Hex(da.Commitment.Marshal()),
		Data:            common.Bytes2Hex(da.Data),
		SignData:        sigDatStr,
		SignAddr:        addrStr,
		ParentStateHash: currentParentHash.Hex(),
		StateHash:       stateHash.Hex(),
		ReceiveAt:       da.ReceiveAt.Format(time.RFC3339),
	}
	res := tx.Create(&wd)
	return res.Error
}

func SaveBatchCommitment(db *gorm.DB, das []*types.DA, parentHash common.Hash) error {
	currentParentHash := parentHash
	dataCollect := make([]byte, 0)
	wdas := make([]DA, 0)

	// 遍历每个区块，依次插入数据库
	for _, da := range das {
		dataCollect = append(dataCollect, da.Commitment.X.Marshal()...)
		dataCollect = append(dataCollect, da.Commitment.Y.Marshal()...)
		dataCollect = append(dataCollect, da.Sender.Bytes()...)
		dataCollect = append(dataCollect, currentParentHash.Bytes()...)
		stateHash := common.BytesToHash(dataCollect)
		commitData := da.Commitment.Marshal()
		sigDatStr := make([]string, len(da.SignData))
		for i,data :=range da.SignData {
			sigDatStr[i] = common.Bytes2Hex(data)
		}
		addrStr := make([]string,len(da.SignerAddr))
		for i,addr := range da.SignerAddr{
			addrStr[i] = addr.Hex()
		}

		wda := DA{
			Sender:          da.Sender.Hex(),
			TxHash:          da.TxHash.String(),
			Index:           int64(da.Index),
			Length:          int64(da.Length),
			Data:            common.Bytes2Hex(da.Data),
			Commitment:      common.Bytes2Hex(commitData),
			CommitmentHash:  common.BytesToHash(commitData).Hex(),
			SignData:        sigDatStr,
			SignAddr:        addrStr,
			ParentStateHash: currentParentHash.String(),
			StateHash:       stateHash.Hex(),
			ReceiveAt:       da.ReceiveAt.Format(time.RFC3339),
		}
		wdas = append(wdas, wda)
		currentParentHash = stateHash
	}

	result := db.Create(&wdas)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func AddBatchCommitment(tx *gorm.DB, das []*types.DA, parentHash common.Hash) error {
	currentParentHash := parentHash
	dataCollect := make([]byte, 0)
	// 遍历每个区块，依次插入数据库
	for _, da := range das {
		dataCollect = append(dataCollect, da.Commitment.X.Marshal()...)
		dataCollect = append(dataCollect, da.Commitment.Y.Marshal()...)
		dataCollect = append(dataCollect, da.Sender.Bytes()...)
		dataCollect = append(dataCollect, currentParentHash.Bytes()...)
		stateHash := common.BytesToHash(dataCollect)
		commitData := da.Commitment.Marshal()
		sigDatStr := make([]string, len(da.SignData))
		for i,data :=range da.SignData {
			sigDatStr[i] = common.Bytes2Hex(data)
		}
		addrStr := make([]string,len(da.SignerAddr))
		for i,addr := range da.SignerAddr{
			addrStr[i] = addr.Hex()
		}
		wda := DA{
			Sender:          da.Sender.Hex(),
			TxHash:          da.TxHash.String(),
			Index:           int64(da.Index),
			Length:          int64(da.Length),
			Data:            common.Bytes2Hex(da.Data),
			Commitment:      common.Bytes2Hex(commitData),
			CommitmentHash:  common.BytesToHash(commitData).Hex(),
			SignData:        sigDatStr,
			SignAddr:        addrStr,
			ParentStateHash: currentParentHash.String(),
			StateHash:       stateHash.Hex(),
			ReceiveAt:       da.ReceiveAt.Format(time.RFC3339),
		}
		log.Info("AddBatchCommitment----","CommitmentHash",wda.CommitmentHash,"TxHash",wda.TxHash)
		result := tx.Create(&wda)
		if result.Error != nil {
			// 插入失败，回滚事务并返回错误
			tx.Rollback()
			return result.Error
		}
		currentParentHash = stateHash
	}
	// 提交事务
	return nil
}

func GetDAByCommitment(db *gorm.DB, commitment []byte) (*types.DA, error) {
	var gormdb *gorm.DB
	var count int64
	gormdb = db.Model(&DA{}).Count(&count)
	if gormdb.Error != nil {
		log.Error("Error count DA", "err", gormdb.Error)
		return nil, gormdb.Error
	}
	if count == 0 {
		msg := fmt.Sprintf("DA table is empty")
		log.Info(msg)
		return nil, errors.New(msg)
	}

	var digest kzg.Digest
	digest.SetBytes(commitment)
	var da DA
	gormdb = db.First(&da, "commitment = ?", common.Bytes2Hex(digest.Marshal()))
	if gormdb.Error != nil {
		log.Error("can not find DA with given commitment", "commitment", common.Bytes2Hex(commitment), "err", gormdb.Error)
		return nil, gormdb.Error
	}

	str, err := hex.DecodeString(da.Commitment)
	if err != nil {
		return nil, err
	}
	_, err = digest.SetBytes(str)
	if err != nil {
		return nil, err
	}
	parsedTime, err := time.Parse(time.RFC3339, da.ReceiveAt)
	if err != nil {
		log.Debug("Error parsing time", "err", err)
		return nil, err
	}
	signData := make([][]byte, len(da.SignData))
	for i,sg := range da.SignData{
		signData[i] = common.Hex2Bytes(sg)
	}

	signAdd := make([]common.Address,len(da.SignAddr))
	for i,add := range da.SignAddr{
		signAdd[i] = common.HexToAddress(add)
	}

	return &types.DA{
		Sender:     common.HexToAddress(da.Sender),
		Index:      uint64(da.Index),
		Length:     uint64(da.Length),
		Commitment: digest,
		Data:       common.Hex2Bytes(da.Data),
		SignData:   signData,
		SignerAddr: signAdd,
		TxHash:     common.HexToHash(da.TxHash),
		ReceiveAt:  parsedTime,
	}, nil
}

func GetDAByCommitmentHash(db *gorm.DB, cmHash common.Hash) (*types.DA, error) {
	var gormdb *gorm.DB
	var count int64
	gormdb = db.Model(&DA{}).Count(&count)
	if gormdb.Error != nil {
		log.Error("Error count DA", "err", gormdb.Error)
		return nil, gormdb.Error
	}
	if count == 0 {
		msg := fmt.Sprintf("DA table is empty")
		log.Info(msg)
		return nil, errors.New(msg)
	}
	var da DA
	gormdb = db.First(&da, "commitment_hash = ?", cmHash.Hex())
	if gormdb.Error != nil {
		log.Error("can not find DA with given commitment_hash", "commitment_hash", cmHash.Hex(), "err", gormdb.Error)
		return nil, gormdb.Error
	}

	var digest kzg.Digest
	str, err := hex.DecodeString(da.Commitment)
	if err != nil {
		return nil, err
	}
	_, err = digest.SetBytes(str)
	if err != nil {
		return nil, err
	}
	parsedTime, err := time.Parse(time.RFC3339, da.ReceiveAt)
	if err != nil {
		log.Debug("Error parsing time", "err", err)
		return nil, err
	}
	signData := make([][]byte, len(da.SignData))
	for i,sg := range da.SignData{
		signData[i] = common.Hex2Bytes(sg)
	}

	signAdd := make([]common.Address,len(da.SignAddr))
	for i,add := range da.SignAddr{
		signAdd[i] = common.HexToAddress(add)
	}
	return &types.DA{
		Sender:     common.HexToAddress(da.Sender),
		Index:      uint64(da.Index),
		Length:     uint64(da.Length),
		Commitment: digest,
		Data:       common.Hex2Bytes(da.Data),
		SignData:   signData,
		SignerAddr: signAdd,
		TxHash:     common.HexToHash(da.TxHash),
		ReceiveAt:  parsedTime,
	}, nil
}

func GetCommitmentByTxHash(db *gorm.DB, txHash common.Hash) (*types.DA, error) {
	var gormdb *gorm.DB

	var count int64
	gormdb = db.Model(&DA{}).Count(&count)
	if gormdb.Error != nil {
		log.Error("Error count DA", "err", gormdb.Error)
		return nil, gormdb.Error
	}
	if count == 0 {
		msg := fmt.Sprintf("DA table is empty")
		log.Info(msg)
		return nil, errors.New(msg)
	}
	var da DA
	gormdb = db.First(&da, "tx_hash = ?", txHash.Hex())
	if gormdb.Error != nil {
		log.Error("can not find DA with given txHash", "txHash", txHash.Hex(), "err", gormdb.Error)
		return nil, gormdb.Error
	}

	var digest kzg.Digest
	str, err := hex.DecodeString(da.Commitment)
	if err != nil {
		return nil, err
	}
	_, err = digest.SetBytes(str)
	if err != nil {
		return nil, err
	}
	parsedTime, err := time.Parse(time.RFC3339, da.ReceiveAt)
	if err != nil {
		log.Debug("Error parsing time", "err", err)
		return nil, err
	}
	signData := make([][]byte, len(da.SignData))
	for i,sg := range da.SignData{
		signData[i] = common.Hex2Bytes(sg)
	}

	signAdd := make([]common.Address,len(da.SignAddr))
	for i,add := range da.SignAddr{
		signAdd[i] = common.HexToAddress(add)
	}
	return &types.DA{
		Sender:     common.HexToAddress(da.Sender),
		Index:      uint64(da.Index),
		Length:     uint64(da.Length),
		Commitment: digest,
		Data:       common.Hex2Bytes(da.Data),
		SignData:   signData,
		SignerAddr: signAdd,
		TxHash:     common.HexToHash(da.TxHash),
		ReceiveAt:  parsedTime,
	}, nil
}

// 获取ID最大的DA记录
func GetMaxIDDAStateHash(db *gorm.DB) (string, error) {
	var da DA
	if err := db.Order("id DESC").First(&da).Error; err != nil {
		return "", err
	}
	return da.StateHash, nil
}

func DeleteDAByHash(db *gorm.DB, hash common.Hash) error {
	var da DA
	tx := db.Where("tx_hash = ?", hash)
	if tx.Error != nil {
		tx = db.Where("commitment = ?", hash)
	}
	err := tx.Delete(&da).Error
	if err != nil {
		db.Rollback()
		return err
	}
	return nil
}

func GetAllDARecords(db *gorm.DB) ([]*types.DA, error) {
	var daRecords []DA
	tx := db.Select("tx_hash", "commitment").Find(&daRecords)
	if tx.Error != nil {
		return nil, tx.Error
	}

	var das []*types.DA
	for _, da := range daRecords {
		var digest kzg.Digest
		str, _ := hex.DecodeString(da.Commitment)
		digest.SetBytes(str)
		parsedTime, err := time.Parse(time.RFC3339, da.ReceiveAt)
		if err != nil {
			fmt.Println("Error parsing time:", err)
		}
		das = append(das, &types.DA{
			TxHash:     common.HexToHash(da.TxHash),
			Commitment: digest,
			ReceiveAt:  parsedTime,
		})
	}
	return das, nil
}
