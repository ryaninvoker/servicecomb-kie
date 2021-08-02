/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//package dao supply pure persistence layer access
package datasource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apache/servicecomb-kie/pkg/model"
	"github.com/apache/servicecomb-kie/server/config"
	"github.com/go-chassis/openlog"
)

var (
	b       Broker
	plugins = make(map[string]New)
)

var (
	ErrKeyNotExists     = errors.New("can not find any key value")
	ErrRecordNotExists  = errors.New("can not find any polling data")
	ErrRevisionNotExist = errors.New("revision does not exist")
	ErrAliasNotGiven    = errors.New("label alias not given")
	ErrKVAlreadyExists  = errors.New("kv already exists")
	ErrTooMany          = errors.New("key with labels should be only one")
)

const (
	DefaultValueType = "text"
)

//New init db session
type New func(c *Config) (Broker, error)

func RegisterPlugin(name string, f New) {
	plugins[name] = f
}

//Broker avoid directly depend on one kind of persistence solution
type Broker interface {
	GetRevisionDao() RevisionDao
	GetHistoryDao() HistoryDao
	GetTrackDao() TrackDao
	GetKVDao() KVDao
}

func GetBroker() Broker {
	return b
}

//KVDao provide api of KV entity
type KVDao interface {
	//below 3 methods is usually for admin console
	Create(ctx context.Context, kv *model.KVDoc) (*model.KVDoc, error)
	Update(ctx context.Context, kv *model.UpdateKVRequest) (*model.KVDoc, error)
	List(ctx context.Context, domain, project string, options ...FindOption) (*model.KVResponse, error)
	//FindOneAndDelete deletes one kv by id and return the deleted kv as these appeared before deletion
	FindOneAndDelete(ctx context.Context, kvID string, domain, project string) (*model.KVDoc, error)
	//FindManyAndDelete deletes multiple kvs and return the deleted kv list as these appeared before deletion
	FindManyAndDelete(ctx context.Context, kvIDs []string, domain, project string) ([]*model.KVDoc, error)
	//Get return kv by id
	Get(ctx context.Context, request *model.GetKVRequest) (*model.KVDoc, error)
	//KVDao is a resource of kie, this api should return kv resource number by domain id
	Total(ctx context.Context, domain string) (int64, error)
}

//HistoryDao provide api of HistoryDao entity
type HistoryDao interface {
	GetHistory(ctx context.Context, keyID string, options ...FindOption) (*model.KVResponse, error)
}

//TrackDao provide api of Track entity
type TrackDao interface {
	CreateOrUpdate(ctx context.Context, detail *model.PollingDetail) (*model.PollingDetail, error)
	GetPollingDetail(ctx context.Context, detail *model.PollingDetail) ([]*model.PollingDetail, error)
}

//RevisionDao is global revision number management
type RevisionDao interface {
	GetRevision(ctx context.Context, domain string) (int64, error)
}

//ViewDao create update and get view data
type ViewDao interface {
	Create(ctx context.Context, viewDoc *model.ViewDoc, options ...FindOption) error
	Update(ctx context.Context, viewDoc *model.ViewDoc) error
	//TODO
	List(ctx context.Context, domain, project string, options ...FindOption) ([]*model.ViewDoc, error)
	GetCriteria(ctx context.Context, viewName, domain, project string) (map[string]map[string]string, error)
	GetContent(ctx context.Context, id, domain, project string, options ...FindOption) ([]*model.KVResponse, error)
}

const DefaultTimeout = 5 * time.Second

func Init(c config.DB) error {
	var err error
	if c.Kind == "" {
		c.Kind = "mongo"
	}
	f, ok := plugins[c.Kind]
	if !ok {
		return fmt.Errorf("do not support %s", c.Kind)
	}
	var timeout time.Duration
	if c.Timeout != "" {
		timeout, err = time.ParseDuration(c.Timeout)
		if err != nil {
			return errors.New("timeout setting invalid:" + c.Timeout)
		}
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	dbc := &Config{
		URI:        c.URI,
		PoolSize:   c.PoolSize,
		SSLEnabled: c.SSLEnabled,
		RootCA:     c.RootCA,
		Timeout:    timeout,
	}
	if b, err = f(dbc); err != nil {
		return err
	}
	openlog.Info(fmt.Sprintf("use %s as storage", c.Kind))
	return nil
}