// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples"
	"github.com/ava-labs/hypersdk/x/programs/examples/simulator/cmd"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	xutils "github.com/ava-labs/hypersdk/x/programs/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type ProgramSimulator struct {
	log logging.Logger
	db  database.Database
}

func newProgramPublish(log logging.Logger, db database.Database) *ProgramSimulator {
	return &ProgramSimulator{
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
	db, _, err := pebble.New(examples.DBPath, pebble.NewDefaultConfig())
	if err != nil {
		return
	}
	utils.Outf("{{yellow}}database:{{/}} %s\n", examples.DBPath)
	programPublish := newProgramPublish(log, db)
	r.Use(cors.New(config))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/api/keys", programPublish.keysHandler)
	r.GET("/api/programs", programPublish.programsHandler)
	r.POST("/api/publish", programPublish.publishHandler)
	r.POST("/api/invoke", programPublish.invokeHandler)

	r.Run(":8080")
}

func (r ProgramSimulator) keysHandler(c *gin.Context) {
	// get keys
	keys, err := cmd.GetKeys(r.db, 5)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"message": "success",
		"keys":    keys,
	})
}

func (r ProgramSimulator) programsHandler(c *gin.Context) {
	// get keys
	programsData, err := cmd.GetPrograms(r.db, r.log)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"message":  "success",
		"programs": programsData,
	})
}

func (r ProgramSimulator) publishHandler(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	num, funcs, err := r.PublishProgram(data)
	// bytes
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Data received successfully", "function_data": funcs, "id": num})
}

func (r ProgramSimulator) invokeHandler(c *gin.Context) {
	var data map[string]interface{}
	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Process the data
	name, nameExists := data["name"].(string)
	if !nameExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'name' field"})
		return
	}

	// params is an array of strings
	params, paramsExists := data["params"].([]interface{})
	if !paramsExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'params' field"})
		return
	}
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
	result, gas, err := r.invokeProgram(programID, name, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data received and processed successfully", "result": result, "gas": gas})
}

func (r ProgramSimulator) PublishProgram(programBytes []byte) (uint64, map[string]int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(r.log, runtime.NewMeter(r.log, examples.DefaultMaxFee, examples.CostMap), r.db)
	defer runtime.Stop(ctx)

	programID, err := runtime.Create(ctx, programBytes)
	if err != nil {
		return 0, nil, err
	}
	data, err := runtime.GetUserData()
	if err != nil {
		return 0, nil, err
	}

	return programID, data, nil
}

func (r ProgramSimulator) invokeProgram(programID uint64, functionName string, params []interface{}) (uint64, uint64, error) {
	exists, program, err := runtime.GetProgram(r.db, programID)
	if !exists {
		return 0, 0, fmt.Errorf("program %v does not exist", programID)
	}
	if err != nil {
		return 0, 0, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: owner for now, change to caller later
	runtime := runtime.New(r.log, runtime.NewMeter(r.log, examples.DefaultMaxFee, examples.CostMap), r.db)
	defer runtime.Stop(ctx)

	err = runtime.Initialize(ctx, program)
	if err != nil {
		return 0, 0, err
	}
	stringParams := make([]string, len(params))
	for i, v := range params {
		if str, ok := v.(string); ok {
			stringParams[i] = str
		} else {
			return 0, 0, fmt.Errorf("invalid type for param %v", i)
		}
	}
	callParams, err := cmd.ParseStringParams(runtime, programID, ctx, stringParams)
	if err != nil {
		return 0, 0, err
	}

	resp, err := runtime.Call(ctx, functionName, callParams...)
	if err != nil {
		return 0, 0, err
	}

	return resp[0], runtime.GetCurrentGas(ctx), nil
}
