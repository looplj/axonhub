package objects

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

type GUID struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

func (guid GUID) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(fmt.Sprintf("gid://%s/%d", guid.Type, guid.ID)))
}

func (guid *GUID) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enum %T must be a string", v)
	}

	if str == "" {
		return errors.New("guid is empty")
	}

	if !strings.HasPrefix(str, "gid://") {
		return errors.New("guid must start with gid://")
	}

	str = str[6:]

	idx := strings.Index(str, "/")
	if idx == -1 {
		return errors.New("guid must contain type and id")
	}

	typ := str[:idx]

	id, err := strconv.Atoi(str[idx+1:])
	if err != nil {
		return err
	}

	guid.Type = typ
	guid.ID = id

	return nil
}

func ParseGUID(str string) (GUID, error) {
	var guid GUID

	err := guid.UnmarshalGQL(str)
	if err != nil {
		return GUID{}, err
	}

	return guid, nil
}

func ConvertGUIDToInt(guid GUID) (int, error) {
	return guid.ID, nil
}

func ConvertGUIDPtrToInt(guid *GUID) (int, error) {
	if guid == nil {
		return 0, errors.New("guid is nil")
	}

	return guid.ID, nil
}

func ConvertGUIDToIntPtr(guid GUID) (*int, error) {
	return lo.ToPtr(guid.ID), nil
}

func ConvertGUIDPtrToIntPtr(guid *GUID) (*int, error) {
	if guid == nil {
		return nil, errors.New("guid is nil")
	}

	return lo.ToPtr(guid.ID), nil
}

func ConvertGUIDPtrsToIntPtrs(guid []*GUID) ([]*int, error) {
	return lo.Map(guid, func(item *GUID, index int) *int {
		return lo.ToPtr(item.ID)
	}), nil
}

func ConvertGUIDPtrsToInts(guid []*GUID) ([]int, error) {
	return lo.Map(guid, func(item *GUID, index int) int {
		return item.ID
	}), nil
}
