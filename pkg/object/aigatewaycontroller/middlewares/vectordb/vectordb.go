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

package vectordb

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/pgvector"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/redisvector"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

var ErrSimilaritySearchNotFound = vecdbtypes.ErrSimilaritySearchNotFound

type (
	Spec struct {
		vecdbtypes.CommonSpec
		Redis    *redisvector.RedisVectorDBSpec `json:"redis,omitempty"`
		Postgres *pgvector.PostgresVectorDBSpec `json:"postgres,omitempty"`
	}

	VectorHandler = vecdbtypes.VectorHandler

	VectorDB = vecdbtypes.VectorDB

	Option  = vecdbtypes.Option
	Options = vecdbtypes.Options
)

const TypeRedis = "redis"
const TypePostgres = "postgres"

func New(spec *Spec) vecdbtypes.VectorDB {
	switch spec.Type {
	case TypeRedis:
		return redisvector.New(&spec.CommonSpec, spec.Redis)
	case TypePostgres:
		return pgvector.New(&spec.CommonSpec, spec.Postgres)
	default:
		panic("not supported vector db type")
	}
}

func ValidateSpec(spec *Spec) error {
	if spec.Threshold <= 0 || spec.Threshold > 1.0 {
		return fmt.Errorf("invalid threshold")
	}
	switch spec.Type {
	case TypeRedis:
		return redisvector.ValidateSpec(spec.Redis)
	case TypePostgres:
		return pgvector.ValidateSpec(spec.Postgres)
	default:
		return fmt.Errorf("invalid spec type")
	}
}
