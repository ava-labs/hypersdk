package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	xutils "github.com/ava-labs/hypersdk/x/programs/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	dbPath = ".serverDb"
)

type ProgramPublish struct {
	log logging.Logger
	db  database.Database
}

func newProgramPublish(log logging.Logger, db database.Database) *ProgramPublish {
	return &ProgramPublish{
		log: log,
		db:  db,
	}
}

func main() {
	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000"} // Update with your frontend URL

	// set log and db
	log := xutils.NewLoggerWithLogLevel(logging.Debug)
	db, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return
	}
	utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
	programPublish := newProgramPublish(log, db)
	r.Use(cors.New(config))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/api/publish", programPublish.publishHandler)

	r.Run(":8080")
}

func (r ProgramPublish) publishHandler(c *gin.Context) {
	data, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	num, err := r.PublishProgram(data)
	// bytes
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println("NUM: ", num)

	c.JSON(http.StatusOK, gin.H{"message": "Data received successfully"})
}

func (r ProgramPublish) PublishProgram(programBytes []byte) (uint64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(r.log, runtime.NewMeter(r.log, maxGas, costMap), r.db, runtimePublicKey)
	defer runtime.Stop(ctx)

	programID, err := runtime.Create(ctx, programBytes)
	if err != nil {
		return 0, err
	}
	return programID, nil
}

var (
	runtimePublicKey = ed25519.EmptyPublicKey
	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	maxGas uint64 = 13000
)
