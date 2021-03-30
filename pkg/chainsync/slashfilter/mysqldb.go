package slashfilter

import (
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus/pkg/config"
)

var log = logging.Logger("mysql")

type mysqlBlockHeader struct {
	Key   string `gorm:"column:key;type:varchar(256);uniqueIndex;NOT NULL"`
	Value []byte `gorm:"column:value;type:blob;"`
}

type MysqlRepo struct {
	_db *gorm.DB
}

func InitMysqlRepo(cfg config.MySQLConfig) (ds.Batching, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DbName,
		"10s")

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, xerrors.Errorf("[db connection failed] Database name: %s %w", cfg.DbName, err)
	}

	db.Set("gorm:table_options", "CHARSET=utf8mb4")
	if cfg.Debug {
		db = db.Debug()
	}

	if err := db.AutoMigrate(mysqlBlockHeader{}); err != nil {
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
	sqlDB.SetConnMaxLifetime(time.Minute * cfg.ConnMaxLifeTime)

	log.Info("init mysql success for SlashFilter!")
	return &MysqlRepo{
		_db: db,
	}, nil
}

func (mr *MysqlRepo) Get(key ds.Key) ([]byte, error) {
	var res mysqlBlockHeader
	if err := mr._db.Where("`key` = ?", key.String()).First(&res).Error; err != nil {
		return nil, err
	}

	return res.Value, nil
}

func (mr *MysqlRepo) Has(key ds.Key) (bool, error) {
	var count int64
	err := mr._db.Model(&mysqlBlockHeader{}).Where("`key` = ?", key.String()).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (mr *MysqlRepo) GetSize(key ds.Key) (int, error) {
	var count int64
	err := mr._db.Model(&mysqlBlockHeader{}).Where("`key` = ?", key.String()).Count(&count).Error
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (mr *MysqlRepo) Query(q query.Query) (query.Results, error) {
	// ToDo implement?

	return nil, nil
}

func (mr *MysqlRepo) Put(key ds.Key, value []byte) error {
	err := mr._db.Create(&mysqlBlockHeader{Key: key.String(), Value: value}).Error
	if err != nil {
		return err
	}

	return nil
}

func (mr *MysqlRepo) Delete(key ds.Key) error {
	var res mysqlBlockHeader
	mr._db.Where("`key` = ?", key.String()).Take(&res)

	return mr._db.Delete(&res).Error
}

func (mr *MysqlRepo) Sync(prefix ds.Key) error {
	// ToDo implement?

	return nil
}

func (mr *MysqlRepo) Close() error {

	return nil
}

func (mr *MysqlRepo) Batch() (ds.Batch, error) {
	// ToDo implement?

	return nil, nil
}
