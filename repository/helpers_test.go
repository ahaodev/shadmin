package repository

import (
	"errors"
	"testing"

	"shadmin/ent"
)

func TestIsUniqueConstraintOn(t *testing.T) {
	for _, tt := range []struct {
		name, message string
		want          bool
	}{
		{"sqlite", "UNIQUE constraint failed: dict_items.type_id, dict_items.label", true},
		{"postgres", `pq: duplicate key value violates unique constraint "dictitem_type_id_label"`, true},
		{"mysql", "Error 1062 (23000): Duplicate entry 'type-label' for key 'dictitem_type_id_label'", true},
		{"mysql qualified key", "Error 1062 (23000): Duplicate entry 'type-label' for key 'dict_items.dictitem_type_id_label'", true},
		{"sqlite other index", "UNIQUE constraint failed: dict_items.type_id, dict_items.value", false},
		{"postgres other index", `pq: duplicate key value violates unique constraint "dictitem_type_id_value"`, false},
		{"mysql index name in value", "Error 1062 (23000): Duplicate entry 'dictitem_type_id_label' for key 'dictitem_type_id_value'", false},
		{"foreign key", "FOREIGN KEY constraint failed", false},
		{"not null", "NOT NULL constraint failed: dict_items.label", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.Join(errors.New(tt.message), &ent.ConstraintError{})
			if got := isUniqueConstraintOn(err, "dict_items.type_id, dict_items.label", "dictitem_type_id_label"); got != tt.want {
				t.Fatalf("isUniqueConstraintOn(%v) = %v; want %v", err, got, tt.want)
			}
		})
	}
	if isUniqueConstraintOn(nil, "dict_items.type_id, dict_items.label", "dictitem_type_id_label") {
		t.Fatal("nil error matched unique constraint")
	}
}
