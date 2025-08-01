/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package redisvector

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"unsafe"

	"github.com/google/uuid"
	"github.com/redis/rueidis"
)

type (
	RedisClient struct {
		client rueidis.Client
	}
)

// NewRedisClient creates a new Redis client with the given options.
func NewRedisClient(opt rueidis.ClientOption) (*RedisClient, error) {
	client, err := rueidis.NewClient(opt)
	if err != nil {
		return nil, err
	}
	return &RedisClient{client: client}, nil
}

// DropIndex drops the index with the given name.
func (c *RedisClient) DropIndex(ctx context.Context, index string, deleteDocuments bool) error {
	if deleteDocuments {
		return c.client.Do(ctx, c.client.B().FtDropindex().Index(index).Dd().Build()).Error()
	}
	return c.client.Do(ctx, c.client.B().FtDropindex().Index(index).Build()).Error()
}

// CheckIndexExists checks if the index with the given name exists.
func (c *RedisClient) CheckIndexExists(ctx context.Context, index string) bool {
	if index == "" {
		return false
	}
	return c.client.Do(ctx, c.client.B().FtInfo().Index(index).Build()).Error() == nil
}

// CreateIndexIfNotExists creates the index with the given name if it does not exist.
func (c *RedisClient) CreateIndexIfNotExists(ctx context.Context, index string, schema *IndexSchema) error {
	if index == "" {
		return errors.New("empty index name")
	}

	if c.CheckIndexExists(ctx, index) {
		return nil
	}

	redisIndex := &Index{
		Name:      index,
		Schema:    schema,
		Prefix:    []string{getPrefix(index)},
		IndexType: "HASH",
	}

	command := redisIndex.ToCommand()
	return c.client.Do(ctx, c.client.B().Arbitrary(command.Commands...).Keys(command.Keys...).Args(command.Args...).Build()).Error()
}

func getPrefix(index string) string {
	return fmt.Sprintf("%s:", index)
}

// InsertWithHash inserts a single document into the index with the given name.
func (c *RedisClient) InsertWithHash(ctx context.Context, index string, doc map[string]any) (string, error) {
	command := toHmsetCommand(index, doc)
	return command.Keys[0], c.client.Do(ctx, c.client.B().Arbitrary(command.Commands...).Keys(command.Keys...).Args(command.Args...).Build()).Error()
}

// InsertManyWithHash inserts multiple documents into the index with the given name.
func (c *RedisClient) InsertManyWithHash(ctx context.Context, index string, docs []map[string]any) ([]string, error) {
	commands := make([]rueidis.Completed, 0, len(docs))
	docIDs := make([]string, 0, len(docs))
	errs := make([]error, 0, len(docs))

	for _, doc := range docs {
		command := toHmsetCommand(index, doc)
		docIDs = append(docIDs, command.Keys[0])
		commands = append(commands, c.client.B().Arbitrary(command.Commands...).Keys(command.Keys...).Args(command.Args...).Build())
	}

	result := c.client.DoMulti(ctx, commands...)
	for _, res := range result {
		if res.Error() != nil {
			errs = append(errs, res.Error())
		}
	}

	return docIDs, errors.Join(errs...)
}

// Find retrieves documents from the index based on the provided query.
func (c *RedisClient) Find(ctx context.Context, query *RedisVectorQuery) (int64, []map[string]any, error) {
	command := query.ToCommand()
	total, docs, err := c.client.Do(ctx, c.client.B().Arbitrary(command.Commands...).Keys(command.Keys...).Args(command.Args...).Build()).AsFtSearch()
	if err != nil {
		return 0, nil, err
	}
	return total, convertFTSearchResIntoMapSchema(docs), nil
}

func convertFTSearchResIntoMapSchema(docs []rueidis.FtSearchDoc) []map[string]any {
	result := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		docMap := make(map[string]any)
		for k, field := range doc.Doc {
			if k == "distance" {
				score, _ := strconv.ParseFloat(field, 32)
				docMap["score"] = float32(score)
			} else {
				docMap[k] = field
			}
		}
		if _, ok := docMap["id"]; !ok {
			docMap["id"] = doc.Key
		}
		result = append(result, docMap)
	}
	return result
}

func toHmsetCommand(prefix string, doc map[string]any) *RedisArbitraryCommand {
	command := &RedisArbitraryCommand{
		Commands: []string{"HMSET"},
	}

	command.Args = make([]string, 0, len(doc)*2)
	for key, value := range doc {
		switch v := value.(type) {
		case []float64:
			command.Args = append(command.Args, key, float64VectorToString(v))
		case []float32:
			command.Args = append(command.Args, key, float32VectorToString(v))
		default:
			command.Args = append(command.Args, key, fmt.Sprintf("%v", v))
		}
	}

	if idx, ok := doc["id"]; ok {
		command.Keys = []string{fmt.Sprintf("%s:%s", prefix, idx)}
	} else if keys, ok := doc["keys"]; ok {
		doc["id"] = keys
		command.Keys = []string{fmt.Sprintf("%s:%s", prefix, keys)}
	} else {
		uuidx := uuid.New().String()
		doc["id"] = uuidx
		command.Keys = []string{fmt.Sprintf("%s:%s", prefix, uuidx)}
	}
	return command
}

func float32VectorToString(v []float32) string {
	b := make([]byte, len(v)*4)
	for i, e := range v {
		i := i * 4
		binary.LittleEndian.PutUint32(b[i:i+4], math.Float32bits(e))
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func float64VectorToString(v []float64) string {
	b := make([]byte, len(v)*8)
	for i, e := range v {
		i := i * 8
		binary.LittleEndian.PutUint64(b[i:i+8], math.Float64bits(e))
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
