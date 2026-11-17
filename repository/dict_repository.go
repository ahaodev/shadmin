package repository

import (
	"context"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/dictitem"
	"shadmin/ent/dicttype"
	"shadmin/internal/constants"
	"time"
)

func domainStatusToDictTypeStatus(status string) dicttype.Status {
	return dicttype.Status(status)
}

func domainStatusToDictItemStatus(status string) dictitem.Status {
	return dictitem.Status(status)
}

func dictTypeStatusToDomainStatus(status dicttype.Status) string {
	return string(status)
}

func dictItemStatusToDomainStatus(status dictitem.Status) string {
	return string(status)
}

type entDictRepository struct {
	client *ent.Client
}

func NewDictRepository(client *ent.Client) domain.DictRepository {
	return &entDictRepository{
		client: client,
	}
}

func (dr *entDictRepository) convertEntDictTypeToDomain(entType *ent.DictType) *domain.DictType {
	if entType == nil {
		return nil
	}

	return &domain.DictType{
		ID:        entType.ID,
		Code:      entType.Code,
		Name:      entType.Name,
		Status:    dictTypeStatusToDomainStatus(entType.Status),
		Remark:    entType.Remark,
		CreatedAt: entType.CreatedAt,
		UpdatedAt: entType.UpdatedAt,
	}
}

func (dr *entDictRepository) convertEntDictItemToDomain(entItem *ent.DictItem) *domain.DictItem {
	if entItem == nil {
		return nil
	}

	return &domain.DictItem{
		ID:        entItem.ID,
		TypeID:    entItem.TypeID,
		Label:     entItem.Label,
		Value:     entItem.Value,
		Sort:      entItem.Sort,
		IsDefault: entItem.IsDefault,
		Status:    dictItemStatusToDomainStatus(entItem.Status),
		Color:     entItem.Color,
		Remark:    entItem.Remark,
		CreatedAt: entItem.CreatedAt,
		UpdatedAt: entItem.UpdatedAt,
	}
}

func (dr *entDictRepository) CreateType(ctx context.Context, dictType *domain.DictType) error {
	// 预查询提供明确错误，唯一索引兜底并发冲突。
	exists, err := clientFromContext(ctx, dr.client).DictType.Query().
		Where(dicttype.Code(dictType.Code)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return domain.ErrDictTypeCodeExists
	}

	now := time.Now()
	dictType.CreatedAt = now
	dictType.UpdatedAt = now

	created, err := clientFromContext(ctx, dr.client).DictType.
		Create().
		SetCode(dictType.Code).
		SetName(dictType.Name).
		SetStatus(domainStatusToDictTypeStatus(dictType.Status)).
		SetNillableRemark(&dictType.Remark).
		SetCreatedAt(dictType.CreatedAt).
		SetUpdatedAt(dictType.UpdatedAt).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrDictTypeCodeExists
		}
		return err
	}

	dictType.ID = created.ID
	return nil
}

func (dr *entDictRepository) GetTypeByID(ctx context.Context, id string) (*domain.DictType, error) {
	entType, err := clientFromContext(ctx, dr.client).DictType.
		Query().
		Where(dicttype.ID(id)).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDictTypeNotFound
		}
		return nil, err
	}

	return dr.convertEntDictTypeToDomain(entType), nil
}

func (dr *entDictRepository) GetTypeByCode(ctx context.Context, code string) (*domain.DictType, error) {
	entType, err := clientFromContext(ctx, dr.client).DictType.
		Query().
		Where(dicttype.Code(code)).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDictTypeNotFound
		}
		return nil, err
	}

	return dr.convertEntDictTypeToDomain(entType), nil
}

func (dr *entDictRepository) FetchTypes(ctx context.Context, params domain.DictTypeQueryParams) (*domain.PagedResult[*domain.DictType], error) {
	query := clientFromContext(ctx, dr.client).DictType.Query()

	if params.Code != "" {
		query = query.Where(dicttype.CodeContains(params.Code))
	}
	if params.Name != "" {
		query = query.Where(dicttype.NameContains(params.Name))
	}
	if params.Status != "" {
		query = query.Where(dicttype.StatusEQ(domainStatusToDictTypeStatus(params.Status)))
	}
	if params.Keyword != "" {
		query = query.Where(
			dicttype.Or(
				dicttype.CodeContains(params.Keyword),
				dicttype.NameContains(params.Keyword),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	if params.SortBy != "" {
		query = query.Order(ApplySorting(params.SortBy, params.Order, map[string]string{
			"code": dicttype.FieldCode,
			"name": dicttype.FieldName,
		}, dicttype.FieldCreatedAt))
	} else {
		query = query.Order(ent.Desc(dicttype.FieldCreatedAt))
	}

	offset, limit := params.Paginate()
	entTypes, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}

	return domain.NewPagedResult(mapSlice(entTypes, dr.convertEntDictTypeToDomain), total, params.Page, params.PageSize), nil
}

func (dr *entDictRepository) UpdateType(ctx context.Context, id string, updates domain.UpdateDictTypeRequest) error {
	exists, err := clientFromContext(ctx, dr.client).DictType.Query().Where(dicttype.ID(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrDictTypeNotFound
	}

	updateQuery := clientFromContext(ctx, dr.client).DictType.UpdateOneID(id)

	if updates.Code != nil {
		// 唯一索引兜底预查询后的并发冲突。
		if exists, err := clientFromContext(ctx, dr.client).DictType.Query().
			Where(dicttype.And(dicttype.Code(*updates.Code), dicttype.Not(dicttype.ID(id)))).
			Exist(ctx); err != nil {
			return err
		} else if exists {
			return domain.ErrDictTypeCodeExists
		}
		updateQuery = updateQuery.SetCode(*updates.Code)
	}
	if updates.Name != nil {
		updateQuery = updateQuery.SetName(*updates.Name)
	}
	if updates.Status != nil {
		updateQuery = updateQuery.SetStatus(domainStatusToDictTypeStatus(*updates.Status))
	}
	if updates.Remark != nil {
		updateQuery = updateQuery.SetRemark(*updates.Remark)
	}

	if _, err = updateQuery.Save(ctx); err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrDictTypeCodeExists
		}
		return err
	}
	return nil
}

func (dr *entDictRepository) DeleteType(ctx context.Context, id string) error {
	return withEntTransaction(ctx, dr.client, func(ctx context.Context, tx *ent.Tx) error {
		// 检查和删除同事务，防止并发创建孤儿字典项。
		hasItems, err := tx.DictItem.Query().
			Where(dictitem.TypeID(id)).
			Exist(ctx)
		if err != nil {
			return err
		}
		if hasItems {
			return domain.ErrDictTypeHasItems
		}

		if err = tx.DictType.DeleteOneID(id).Exec(ctx); err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrDictTypeNotFound
			}
			return err
		}
		return nil
	})
}

func (dr *entDictRepository) CreateItem(ctx context.Context, dictItem *domain.DictItem) error {
	return withEntTransaction(ctx, dr.client, func(ctx context.Context, tx *ent.Tx) error {
		typeExists, err := tx.DictType.Query().Where(dicttype.ID(dictItem.TypeID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !typeExists {
			return domain.ErrDictTypeNotFound
		}

		valueExists, err := tx.DictItem.Query().
			Where(dictitem.And(dictitem.TypeID(dictItem.TypeID), dictitem.Value(dictItem.Value))).
			Exist(ctx)
		if err != nil {
			return err
		}
		if valueExists {
			return domain.ErrDictItemValueExists
		}

		labelExists, err := tx.DictItem.Query().
			Where(dictitem.And(dictitem.TypeID(dictItem.TypeID), dictitem.LabelEQ(dictItem.Label))).
			Exist(ctx)
		if err != nil {
			return err
		}
		if labelExists {
			return domain.ErrDictItemLabelExists
		}

		if dictItem.IsDefault {
			if _, err = tx.DictItem.Update().
				Where(dictitem.And(dictitem.TypeID(dictItem.TypeID), dictitem.IsDefault(true))).
				SetIsDefault(false).
				Save(ctx); err != nil {
				return err
			}
		}

		now := time.Now()
		dictItem.CreatedAt = now
		dictItem.UpdatedAt = now

		created, err := tx.DictItem.
			Create().
			SetTypeID(dictItem.TypeID).
			SetLabel(dictItem.Label).
			SetValue(dictItem.Value).
			SetSort(dictItem.Sort).
			SetIsDefault(dictItem.IsDefault).
			SetStatus(domainStatusToDictItemStatus(dictItem.Status)).
			SetNillableColor(&dictItem.Color).
			SetNillableRemark(&dictItem.Remark).
			SetCreatedAt(dictItem.CreatedAt).
			SetUpdatedAt(dictItem.UpdatedAt).
			Save(ctx)
		if err != nil {
			return dictItemWriteError(err)
		}

		dictItem.ID = created.ID
		return nil
	})
}

func (dr *entDictRepository) GetItemByID(ctx context.Context, id string) (*domain.DictItem, error) {
	entItem, err := clientFromContext(ctx, dr.client).DictItem.
		Query().
		Where(dictitem.ID(id)).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDictItemNotFound
		}
		return nil, err
	}

	return dr.convertEntDictItemToDomain(entItem), nil
}

func (dr *entDictRepository) FetchItems(ctx context.Context, params domain.DictItemQueryParams) (*domain.PagedResult[*domain.DictItem], error) {
	query := clientFromContext(ctx, dr.client).DictItem.Query()

	if params.TypeID != "" {
		query = query.Where(dictitem.TypeID(params.TypeID))
	} else if params.TypeCode != "" {
		dictType, err := clientFromContext(ctx, dr.client).DictType.Query().
			Where(dicttype.Code(params.TypeCode)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.NewPagedResult([]*domain.DictItem{}, 0, params.Page, params.PageSize), nil
			}
			return nil, err
		}
		query = query.Where(dictitem.TypeID(dictType.ID))
	}

	if params.Label != "" {
		query = query.Where(dictitem.LabelContains(params.Label))
	}
	if params.Value != "" {
		query = query.Where(dictitem.ValueContains(params.Value))
	}
	if params.Status != "" {
		query = query.Where(dictitem.StatusEQ(domainStatusToDictItemStatus(params.Status)))
	}
	if params.Keyword != "" {
		query = query.Where(
			dictitem.Or(
				dictitem.LabelContains(params.Keyword),
				dictitem.ValueContains(params.Keyword),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	if params.SortBy != "" {
		query = query.Order(ApplySorting(params.SortBy, params.Order, map[string]string{
			"label": dictitem.FieldLabel,
			"value": dictitem.FieldValue,
			"sort":  dictitem.FieldSort,
		}, dictitem.FieldCreatedAt))
	} else {
		query = query.Order(ent.Asc(dictitem.FieldSort), ent.Desc(dictitem.FieldCreatedAt))
	}

	offset, limit := params.Paginate()
	entItems, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}

	return domain.NewPagedResult(mapSlice(entItems, dr.convertEntDictItemToDomain), total, params.Page, params.PageSize), nil
}

func (dr *entDictRepository) UpdateItem(ctx context.Context, id string, updates domain.UpdateDictItemRequest) error {
	return withEntTransaction(ctx, dr.client, func(ctx context.Context, tx *ent.Tx) error {
		currentItem, err := tx.DictItem.Query().Where(dictitem.ID(id)).First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrDictItemNotFound
			}
			return err
		}

		updateQuery := tx.DictItem.UpdateOneID(id)

		if updates.Value != nil && *updates.Value != currentItem.Value {
			valueExists, err := tx.DictItem.Query().
				Where(dictitem.And(
					dictitem.TypeID(currentItem.TypeID),
					dictitem.Value(*updates.Value),
					dictitem.Not(dictitem.ID(id)),
				)).
				Exist(ctx)
			if err != nil {
				return err
			}
			if valueExists {
				return domain.ErrDictItemValueExists
			}
			updateQuery = updateQuery.SetValue(*updates.Value)
		}

		if updates.Label != nil && *updates.Label != currentItem.Label {
			labelExists, err := tx.DictItem.Query().
				Where(dictitem.And(
					dictitem.TypeID(currentItem.TypeID),
					dictitem.LabelEQ(*updates.Label),
					dictitem.Not(dictitem.ID(id)),
				)).
				Exist(ctx)
			if err != nil {
				return err
			}
			if labelExists {
				return domain.ErrDictItemLabelExists
			}
			updateQuery = updateQuery.SetLabel(*updates.Label)
		}

		if updates.Sort != nil {
			updateQuery = updateQuery.SetSort(*updates.Sort)
		}
		if updates.Status != nil {
			updateQuery = updateQuery.SetStatus(domainStatusToDictItemStatus(*updates.Status))
		}
		if updates.Color != nil {
			updateQuery = updateQuery.SetColor(*updates.Color)
		}
		if updates.Remark != nil {
			updateQuery = updateQuery.SetRemark(*updates.Remark)
		}

		if updates.IsDefault != nil {
			if *updates.IsDefault && !currentItem.IsDefault {
				if _, err = tx.DictItem.Update().
					Where(dictitem.And(
						dictitem.TypeID(currentItem.TypeID),
						dictitem.IsDefault(true),
						dictitem.Not(dictitem.ID(id)),
					)).
					SetIsDefault(false).
					Save(ctx); err != nil {
					return err
				}
			}
			updateQuery = updateQuery.SetIsDefault(*updates.IsDefault)
		}

		_, err = updateQuery.Save(ctx)
		return dictItemWriteError(err)
	})
}

func dictItemWriteError(err error) error {
	switch {
	case isUniqueConstraintOn(err, "dict_items.type_id, dict_items.label", "dictitem_type_id_label"):
		return domain.ErrDictItemLabelExists
	case isUniqueConstraintOn(err, "dict_items.type_id, dict_items.value", "dictitem_type_id_value"):
		return domain.ErrDictItemValueExists
	default:
		return err
	}
}

func (dr *entDictRepository) DeleteItem(ctx context.Context, id string) error {
	err := clientFromContext(ctx, dr.client).DictItem.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrDictItemNotFound
		}
		return err
	}
	return nil
}

func (dr *entDictRepository) GetItemsByTypeCode(ctx context.Context, typeCode string, status string) ([]*domain.DictItem, error) {
	dictType, err := clientFromContext(ctx, dr.client).DictType.Query().
		Where(dicttype.Code(typeCode)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return []*domain.DictItem{}, nil
		}
		return nil, err
	}

	query := clientFromContext(ctx, dr.client).DictItem.Query().Where(dictitem.TypeID(dictType.ID))

	if status == "" {
		status = constants.StatusActive
	}
	query = query.Where(dictitem.StatusEQ(domainStatusToDictItemStatus(status)))

	entItems, err := query.Order(ent.Asc(dictitem.FieldSort), ent.Desc(dictitem.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}

	return mapSlice(entItems, dr.convertEntDictItemToDomain), nil
}
