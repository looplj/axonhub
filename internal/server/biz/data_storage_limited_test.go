package biz

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

type unexpectedReadReader struct{}

func (unexpectedReadReader) Read([]byte) (int, error) {
	return 0, errors.New("reader should not be consumed")
}

func TestReadDataLimited_RejectsAdvertisedAndObservedOversize(t *testing.T) {
	_, err := readDataLimited(unexpectedReadReader{}, 5, 4)
	require.ErrorIs(t, err, errDataExceedsLimit)

	_, err = readDataLimited(&repeatingReader{remaining: 5}, 1, 4)
	require.ErrorIs(t, err, errDataExceedsLimit)
}

func TestDataStorageService_LoadDataLimited_FileSystem(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "body.json", []byte(`{"ok":true}`), 0o600))

	service := &DataStorageService{fsCache: map[int]afero.Fs{7: fs}}
	storage := &ent.DataStorage{ID: 7, Type: datastorage.TypeFs}

	data, err := service.LoadDataLimited(t.Context(), storage, "body.json", int64(len(`{"ok":true}`)))
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(data))

	data, err = service.LoadDataLimited(t.Context(), storage, "body.json", int64(len(`{"ok":true}`)-1))
	require.Nil(t, data)
	require.ErrorIs(t, err, errDataExceedsLimit)
}

func TestRequestService_LoadCompletedResponseExchangeLimitsExternalBodies(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_history_external_limit?mode=memory&_fk=0")
	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	storage, err := client.DataStorage.Create().
		SetName("history-fs").
		SetDescription("history external storage").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{}).
		Save(ctx)
	require.NoError(t, err)
	stored, err := client.Request.Create().
		SetProjectID(1).
		SetDataStorageID(storage.ID).
		SetModelID("gpt-5.5").
		SetFormat(llm.APIFormatOpenAIResponse.String()).
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetResponseBody(objects.JSONRawMessage(`{}`)).
		SetExternalID("resp_external").
		Save(ctx)
	require.NoError(t, err)

	requestBody := []byte(`{"model":"gpt-5.5","input":"external"}`)
	responseBody := []byte(`{"id":"resp_external","model":"gpt-5.5","output":[]}`)
	fs := afero.NewMemMapFs()
	requestKey := GenerateRequestBodyKey(stored.ProjectID, stored.ID)
	responseKey := GenerateResponseBodyKey(stored.ProjectID, stored.ID)
	require.NoError(t, fs.MkdirAll(filepath.Dir(requestKey), 0o755))
	require.NoError(t, afero.WriteFile(fs, requestKey, requestBody, 0o600))
	require.NoError(t, afero.WriteFile(fs, responseKey, responseBody, 0o600))

	storageService := &DataStorageService{
		AbstractService:  &AbstractService{db: client},
		Cache:            xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
		fsCache:          map[int]afero.Fs{storage.ID: fs},
		objectStoreCache: make(map[int]ObjectStore),
	}
	requestService := &RequestService{
		AbstractService:    &AbstractService{db: client},
		DataStorageService: storageService,
	}

	exactBudget := int64(len(requestBody) + len(responseBody))
	exchange, err := requestService.LoadCompletedResponseExchange(ctx, "resp_external", 1, nil, exactBudget)
	require.NoError(t, err)
	require.Equal(t, objects.JSONRawMessage(requestBody), exchange.RequestBody)
	require.Equal(t, objects.JSONRawMessage(responseBody), exchange.ResponseBody)

	exchange, err = requestService.LoadCompletedResponseExchange(ctx, "resp_external", 1, nil, exactBudget-1)
	require.Nil(t, exchange)
	require.ErrorIs(t, err, ErrStoredResponseExchangeTooLarge)
}

func TestRequestService_LoadCompletedResponseExchange_FindsCompactFormat(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_history_compact_format?mode=memory&_fk=0")
	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	_, err := client.DataStorage.Create().
		SetName("primary-database").
		SetDescription("test primary database storage").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		Save(ctx)
	require.NoError(t, err)

	requestBody := objects.JSONRawMessage(`{"model":"gpt-5.5","instructions":"be brief","input":"compact history"}`)
	responseBody := objects.JSONRawMessage(`{"id":"resp_compact","model":"gpt-5.5","output":[]}`)
	_, err = client.Request.Create().
		SetProjectID(1).
		SetModelID("gpt-5.5").
		SetFormat(llm.APIFormatOpenAIResponseCompact.String()).
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetExternalID("resp_compact").
		Save(ctx)
	require.NoError(t, err)

	requestService := &RequestService{
		AbstractService: &AbstractService{db: client},
		DataStorageService: &DataStorageService{
			AbstractService:  &AbstractService{db: client},
			Cache:            xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
			objectStoreCache: make(map[int]ObjectStore),
		},
	}

	exchange, err := requestService.LoadCompletedResponseExchange(ctx, "resp_compact", 1, nil, 1<<20)
	require.NoError(t, err)
	require.NotNil(t, exchange, "compact-format responses must be discoverable by previous_response_id")
	require.Equal(t, requestBody, exchange.RequestBody)
	require.Equal(t, responseBody, exchange.ResponseBody)
}

type repeatingReader struct {
	remaining int
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, nil
	}
	n := min(len(p), r.remaining)
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= n
	return n, nil
}
