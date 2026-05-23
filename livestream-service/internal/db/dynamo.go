package db

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Store is a light repository abstraction; in-memory maps are used as fallback during local development.
type Store struct {
	StreamsTable string
	ChatTable    string
	TicketsTable string
	ddb          *dynamodb.Client
	useMemory    bool

	mu      sync.RWMutex
	streams map[string]map[string]interface{}
	chat    map[string][]map[string]interface{}
	tickets map[string]map[string]time.Time
}

func NewStore(streamsTable, chatTable, ticketsTable string) *Store {
	return &Store{
		StreamsTable: streamsTable,
		ChatTable:    chatTable,
		TicketsTable: ticketsTable,
		useMemory:    true,
		streams:      map[string]map[string]interface{}{},
		chat:         map[string][]map[string]interface{}{},
		tickets:      map[string]map[string]time.Time{},
	}
}

func NewStoreWithDynamo(ctx context.Context, region, endpointURL, streamsTable, chatTable, ticketsTable string) (*Store, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if endpointURL != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(endpointURL))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Store{
		StreamsTable: streamsTable,
		ChatTable:    chatTable,
		TicketsTable: ticketsTable,
		ddb:          dynamodb.NewFromConfig(cfg),
		useMemory:    false,
		streams:      map[string]map[string]interface{}{},
		chat:         map[string][]map[string]interface{}{},
		tickets:      map[string]map[string]time.Time{},
	}, nil
}

func (s *Store) PutStream(ctx context.Context, id string, item map[string]interface{}) error {
	if !s.useMemory {
		item["stream_id"] = id
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("marshal stream: %w", err)
		}
		_, err = s.ddb.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(s.StreamsTable), Item: av})
		if err != nil {
			return fmt.Errorf("put stream: %w", err)
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.streams[id] = item
	return nil
}

func (s *Store) GetStream(ctx context.Context, id string) (map[string]interface{}, error) {
	if !s.useMemory {
		out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(s.StreamsTable),
			Key: map[string]types.AttributeValue{
				"stream_id": &types.AttributeValueMemberS{Value: id},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("get stream: %w", err)
		}
		if len(out.Item) == 0 {
			return nil, fmt.Errorf("stream not found")
		}
		item := map[string]interface{}{}
		if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
			return nil, fmt.Errorf("unmarshal stream: %w", err)
		}
		return item, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.streams[id]
	if !ok {
		return nil, fmt.Errorf("stream not found")
	}
	return item, nil
}

func (s *Store) DeleteStream(ctx context.Context, id string) error {
	if !s.useMemory {
		_, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.StreamsTable),
			Key: map[string]types.AttributeValue{
				"stream_id": &types.AttributeValueMemberS{Value: id},
			},
		})
		if err != nil {
			return fmt.Errorf("delete stream: %w", err)
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.streams, id)
	return nil
}

func (s *Store) ListStreams(ctx context.Context) ([]map[string]interface{}, error) {
	if !s.useMemory {
		out, err := s.ddb.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(s.StreamsTable)})
		if err != nil {
			return nil, fmt.Errorf("scan streams: %w", err)
		}
		res := make([]map[string]interface{}, 0, len(out.Items))
		for _, item := range out.Items {
			it := map[string]interface{}{}
			if err := attributevalue.UnmarshalMap(item, &it); err == nil {
				res = append(res, it)
			}
		}
		return res, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(s.streams))
	for _, v := range s.streams {
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) PutChatMessage(ctx context.Context, streamID string, msg map[string]interface{}) error {
	if !s.useMemory {
		msg["stream_id"] = streamID
		msg["message_id"] = fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), rand.Intn(1000000))
		av, err := attributevalue.MarshalMap(msg)
		if err != nil {
			return fmt.Errorf("marshal chat message: %w", err)
		}
		_, err = s.ddb.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(s.ChatTable), Item: av})
		if err != nil {
			return fmt.Errorf("put chat message: %w", err)
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.chat[streamID] = append(s.chat[streamID], msg)
	return nil
}

func (s *Store) ChatHistory(ctx context.Context, streamID string, limit int) ([]map[string]interface{}, error) {
	if !s.useMemory {
		out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.ChatTable),
			KeyConditionExpression: aws.String("stream_id = :sid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":sid": &types.AttributeValueMemberS{Value: streamID},
			},
			ScanIndexForward: aws.Bool(false),
			Limit:            aws.Int32(int32(limit)),
		})
		if err != nil {
			return nil, fmt.Errorf("query chat history: %w", err)
		}
		res := make([]map[string]interface{}, 0, len(out.Items))
		for _, item := range out.Items {
			it := map[string]interface{}{}
			if err := attributevalue.UnmarshalMap(item, &it); err == nil {
				res = append(res, it)
			}
		}
		for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
			res[i], res[j] = res[j], res[i]
		}
		return res, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.chat[streamID]
	if len(msgs) <= limit {
		return append([]map[string]interface{}(nil), msgs...), nil
	}
	return append([]map[string]interface{}(nil), msgs[len(msgs)-limit:]...), nil
}

func (s *Store) GrantTicket(ctx context.Context, streamID, userID string, expiresAt time.Time) error {
	if !s.useMemory {
		item := map[string]interface{}{
			"stream_id":      streamID,
			"viewer_user_id": userID,
			"expires_at":     expiresAt.Format(time.RFC3339),
			"ttl":            strconv.FormatInt(expiresAt.Unix(), 10),
		}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("marshal ticket: %w", err)
		}
		_, err = s.ddb.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
			TransactItems: []types.TransactWriteItem{{
				Put: &types.Put{TableName: aws.String(s.TicketsTable), Item: av},
			}},
		})
		if err != nil {
			return fmt.Errorf("grant ticket: %w", err)
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tickets[streamID]; !ok {
		s.tickets[streamID] = map[string]time.Time{}
	}
	s.tickets[streamID][userID] = expiresAt
	return nil
}

func (s *Store) HasTicket(ctx context.Context, streamID, userID string) bool {
	if !s.useMemory {
		out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(s.TicketsTable),
			Key: map[string]types.AttributeValue{
				"stream_id":      &types.AttributeValueMemberS{Value: streamID},
				"viewer_user_id": &types.AttributeValueMemberS{Value: userID},
			},
		})
		if err != nil || len(out.Item) == 0 {
			return false
		}
		ttl := ""
		if v, ok := out.Item["ttl"].(*types.AttributeValueMemberS); ok {
			ttl = v.Value
		}
		if ttl == "" {
			return true
		}
		ts, err := strconv.ParseInt(ttl, 10, 64)
		if err != nil {
			return true
		}
		return ts > time.Now().Unix()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.tickets[streamID][userID]
	return ok && exp.After(time.Now())
}
