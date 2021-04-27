package slashfilter

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus/pkg/config"
)

var log = logging.Logger("mysql")

type MysqlSlashFilter struct {
	_db *gorm.DB
}

//MinedBlock record mined block
type MinedBlock struct {
	ParentEpoch int64  `gorm:"column:parent_epoch;type:bigint(20);NOT NULL"`
	ParentKey   string `gorm:"column:parent_key;type:varchar(256);NOT NULL"`

	Epoch int64  `gorm:"column:epoch;type:bigint(20);NOT NULL"`
	Miner string `gorm:"column:miner;type:varchar(256);NOT NULL"`
	Cid   string `gorm:"column:cid;type:varchar(256);NOT NULL"`
}

//NewMysqlSlashFilter create a new slash filter base on  mysql database
func NewMysqlSlashFilter(cfg config.MySQLConfig) (ISlashFilter, error) {
	db, err := gorm.Open(mysql.Open(cfg.ConnectionString))
	if err != nil {
		return nil, xerrors.Errorf("[db connection failed] Connection : %s %w", cfg.ConnectionString, err)
	}

	if cfg.Debug {
		db = db.Debug()
	}

	if err := db.AutoMigrate(MinedBlock{}); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set the maximum number of idle connections in the connection pool.
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConn)
	// Set the maximum number of open database connections.
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConn)
	// The maximum time that the connection can be reused is set.
	sqlDB.SetConnMaxLifetime(time.Second * cfg.ConnMaxLifeTime)

	log.Info("init mysql success for LocalSlashFilter!")
	return &MysqlSlashFilter{
		_db: db,
	}, nil
}

//checkSameHeightFault check whether the miner mined multi block on the same height
func (f *MysqlSlashFilter) checkSameHeightFault(bh *types.BlockHeader) error {
	var bk MinedBlock
	err := f._db.Model(&MinedBlock{}).Take(&bk, "miner=? and epoch=?", bh.Miner.String(), bh.Height).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}

	other, err := cid.Decode(bk.Cid)
	if err != nil {
		return err
	}

	if other == bh.Cid() {
		return nil
	}

	return xerrors.Errorf("produced block would trigger double-fork mining faults consensus fault; miner: %s; bh: %s, other: %s", bh.Miner, bh.Cid(), other)

}

//checkSameParentFault check whether the miner mined block on the same parent
func (f *MysqlSlashFilter) checkSameParentFault(bh *types.BlockHeader) error {
	var bk MinedBlock
	err := f._db.Model(&MinedBlock{}).Take(&bk, "miner=? and parent_key=?", bh.Miner.String(), bh.Parents.String()).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}

	other, err := cid.Decode(bk.Cid)
	if err != nil {
		return err
	}

	if other == bh.Cid() {
		return nil
	}

	return xerrors.Errorf("produced block would trigger time-offset mining faults consensus fault; miner: %s; bh: %s, other: %s", bh.Miner, bh.Cid(), other)

}

//MinedBlock check whether the block mined is slash
func (f *MysqlSlashFilter) MinedBlock(bh *types.BlockHeader, parentEpoch abi.ChainEpoch) error {
	if err := f.checkSameHeightFault(bh); err != nil {
		return err
	}

	if err := f.checkSameParentFault(bh); err != nil {
		return err
	}

	{
		// parent-grinding fault (didn't mine on top of our own block)

		// First check if we have mined a block on the parent epoch
		var bk MinedBlock
		err := f._db.Model(&MinedBlock{}).Take(&bk, "miner=? and parent_epoch=?", bh.Miner.String(), parentEpoch).Error
		if err == nil {
			//if exit
			parent, err := cid.Decode(bk.Cid)
			if err != nil {
				return err
			}

			var found bool
			for _, c := range bh.Parents.Cids() {
				if c.Equals(parent) {
					found = true
				}
			}

			if !found {
				return xerrors.Errorf("produced block would trigger 'parent-grinding fault' consensus fault; miner: %s; bh: %s, expected parent: %s", bh.Miner, bh.Cid(), parent)
			}
		} else if err != gorm.ErrRecordNotFound {
			//other error except not found
			return err
		}
		//if not exit good block
	}

	return f._db.Save(&MinedBlock{
		ParentEpoch: int64(parentEpoch),
		ParentKey:   bh.Parents.String(),
		Epoch:       int64(bh.Height),
		Miner:       bh.Miner.String(),
		Cid:         bh.Cid().String(),
	}).Error
}
