package model

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReplaceTokenGroupRouteOrderPreservesOrderAndRejectsDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&TokenGroupRoute{}); err != nil {
		t.Fatalf("migrate token group routes: %v", err)
	}

	if err := ReplaceTokenGroupRouteOrderWithDB(db, "token-1", []string{"group-b", "group-a"}, 100); err != nil {
		t.Fatalf("save route order: %v", err)
	}
	got, err := ListTokenGroupRouteOrderWithDB(db, "token-1")
	if err != nil {
		t.Fatalf("list route order: %v", err)
	}
	if want := []string{"group-b", "group-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("route order=%v, want %v", got, want)
	}

	if err := ReplaceTokenGroupRouteOrderWithDB(db, "token-1", []string{"group-a", "group-a"}, 101); err == nil {
		t.Fatal("duplicate group IDs should be rejected")
	}
	got, err = ListTokenGroupRouteOrderWithDB(db, "token-1")
	if err != nil {
		t.Fatalf("list route order after rejected update: %v", err)
	}
	if want := []string{"group-b", "group-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("route order after rejected update=%v, want previous order %v", got, want)
	}
}
