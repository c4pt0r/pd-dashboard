package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"strings"
	"time"

	"github.com/ngaut/log"
	"github.com/ngaut/pool"
)

var (
	rows       = flag.Int("rows", 10000, "row number of bench table, default: 10000")
	concurrent = flag.Int("c", 50, "concurrent workers, default: 50")
	batchSize  = flag.Int("batch", 5000, "batch size, default: 5000")
	bulkSize   = flag.Int("bulk", 20, "test data size (one field, in byte), default: 20")
	poolSize   = flag.Int("pool", 100, "connection poll size, default: 200")
	nCols      = flag.Int("cols", 2, "bench table column number, default: 2")
	tblPrefix  = flag.String("prefix", "", "bench table prefix, default: tidb_{random}")
	addr       = flag.String("addr", ":4000", "tidb-server addr, default: :4000")
	dbName     = flag.String("db", "test", "db name, default: test")
	force      = flag.Bool("f", true, "drop table first")
	user       = flag.String("u", "root", "username, default: root")
	password   = flag.String("p", "", "password, default: empty")
	logLevel   = flag.String("L", "info", "log level, default: error")

	tableName string
)

const (
	forceDrop = true
)

var (
	connPool = pool.NewCache("pool", *poolSize, func() interface{} {
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", *user, *password, *addr, *dbName))
		if err != nil {
			log.Fatal(err)
		}
		return db
	})
)

func init() {
	flag.Parse()
	if len(*tblPrefix) == 0 {
		// if user doesn't provide specific table prefix, we generate one.
		tableName = fmt.Sprintf("tidb_%v_bench", time.Now().UnixNano())
	} else {
		tableName = *tblPrefix + "_bench"
	}
}

func doBatchInsert(ids []int) {
	sqlFmt := "INSERT INTO %s VALUES %s"
	var stmts []string
	for _, i := range ids {
		var strFields []string
		for j := 0; j < *nCols; j++ {
			buf := bytes.Repeat([]byte{'A'}, *bulkSize)
			strFields = append(strFields, "\""+string(buf)+"\"")
		}
		val := fmt.Sprintf("(%d, %s)", i, strings.Join(strFields, ","))
		sql := fmt.Sprintf(sqlFmt, tableName, val)
		stmts = append(stmts, sql)
	}
	err := execTxn(stmts)
	if err != nil {
		log.Fatal(err)
	}
}

func insertTestData(workers int) error {
	jobChan := make(chan int)
	for i := 0; i < workers; i++ {
		go func() {
			var ids []int
			for id := range jobChan {
				ids = append(ids, id)
				if len(ids) == *batchSize {
					doBatchInsert(ids)
					ids = nil
					log.Infof("insert %d record successfully", *batchSize)
				}
			}
		}()
	}
	cur := 0
	for {
		jobChan <- cur
		cur++
	}
}

func main() {
	log.SetLevelByString(*logLevel)
	createTable(forceDrop)
	{
		timing("insert test data", func() {
			insertTestData(*concurrent)
		})
	}
}
