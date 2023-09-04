package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

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
	r.POST("/api/invoke", programPublish.invokeHandler)

	r.Run(":8080")
}

func (r ProgramPublish) publishHandler(c *gin.Context) {
	data, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	num, funcs, err := r.PublishProgram(data)
	// bytes
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println("NUM: ", num)
	fmt.Println("Funs: ", funcs)
	c.JSON(http.StatusOK, gin.H{"message": "Data received successfully", "function_data": funcs, "id": num})
}

func (r ProgramPublish) invokeHandler(c *gin.Context) {
	var data map[string]interface{}
	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println("here")
	// Process the data
	// For example, let's assume the JSON data has a "name" field
	name, nameExists := data["name"].(string)
	if !nameExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'name' field"})
		return
	}

	fmt.Println("Name: ", name)
	// params is an array of strings
	params, paramsExists := data["params"].([]interface{})
	if !paramsExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'params' field"})
		return
	}
	fmt.Println("Params: ", params)
	value, programIDExists := data["programID"].(string)
	if !programIDExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'programID' field"})
		return
	}
	programID, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// now we have the function name, id and the params, can invoke
	fmt.Println("ProgramID: ", programID)
	c.JSON(http.StatusOK, gin.H{"message": "Data received and processed successfully"})
}

func (r ProgramPublish) PublishProgram(programBytes []byte) (uint64, map[string]int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(r.log, runtime.NewMeter(r.log, maxGas, costMap), r.db, runtimePublicKey)
	defer runtime.Stop(ctx)

	programID, err := runtime.Create(ctx, programBytes)
	if err != nil {
		return 0, nil, err
	}
	data, err := runtime.GetUserData(ctx)
	if err != nil {
		return 0, nil, err
	}

	return programID, data, nil
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

// func () {
// 	exists, owner, program, err := runtime.GetProgram(db, programID)
// 		if !exists {
// 			return fmt.Errorf("program %v does not exist", programID)
// 		}
// 		if err != nil {
// 			return err
// 		}

// 		ctx, cancel := context.WithCancel(context.Background())
// 		defer cancel()

// 		// TODO: owner for now, change to caller later
// 		runtime := runtime.New(log, runtime.NewMeter(log, maxFee, costMap), db, owner)
// 		defer runtime.Stop(ctx)

// 		err = runtime.Initialize(ctx, program)
// 		if err != nil {
// 			return err
// 		}

// 		var callParams []uint64
// 		if params != "" {
// 			for _, param := range strings.Split(params, ",") {
// 				switch p := strings.ToLower(param); {
// 				case p == "true":
// 					callParams = append(callParams, 1)
// 				case p == "false":
// 					callParams = append(callParams, 0)
// 				case strings.HasPrefix(p, HRP):
// 					// address
// 					pk, err := getPublicKey(db, p)
// 					if err != nil {
// 						return err
// 					}
// 					ptr, err := runtime.WriteGuestBuffer(ctx, pk[:])
// 					if err != nil {
// 						return err
// 					}
// 					callParams = append(callParams, ptr)
// 				default:
// 					// treat like a number
// 					var num uint64
// 					num, err := strconv.ParseUint(p, 10, 64)

// 					if err != nil {
// 						return err
// 					}
// 					callParams = append(callParams, num)
// 				}
// 			}
// 		}
// 		// prepend programID
// 		callParams = append([]uint64{programID}, callParams...)

// 		resp, err := runtime.Call(ctx, functionName, callParams...)
// 		if err != nil {
// 			return err
// 		}

// 		utils.Outf("{{green}}response:{{/}} %v\n", resp)
// 		return nil
// }
