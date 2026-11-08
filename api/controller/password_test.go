package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/constants"
	"shadmin/repository"
	"shadmin/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordEndpointsEnforceByteLimits(t *testing.T) {
	for _, operation := range []string{"create", "edit", "profile"} {
		for _, test := range []struct {
			name, password string
			valid          bool
		}{
			{"empty", "", false},
			{"short", "1234567", false},
			{"minimum", "12345678", true},
			{"maximum", strings.Repeat("x", 72), true},
			{"over maximum", strings.Repeat("x", 73), false},
			{"UTF8 too few characters", "密码a1", false},
			{"UTF8 short", strings.Repeat("密", 7), false},
			{"UTF8 minimum", strings.Repeat("密", 8), true},
			{"UTF8 maximum", strings.Repeat("密", 24), true},
			{"UTF8 over maximum", strings.Repeat("密", 25), false},
			{"emoji too few characters", strings.Repeat("🔑", 7), false},
			{"emoji minimum", strings.Repeat("🔑", 8), true},
			{"emoji maximum", strings.Repeat("🔑", 18), true},
			{"emoji over maximum", strings.Repeat("🔑", 19), false},
		} {
			t.Run(operation+"/"+test.name, func(t *testing.T) {
				ctx := context.Background()
				client, err := ent.Open("sqlite3", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_fk=1")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = client.Close() })
				if err := client.Schema.Create(ctx); err != nil {
					t.Fatal(err)
				}
				if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
					t.Fatal(err)
				}
				hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
				if err != nil {
					t.Fatal(err)
				}
				repo := repository.NewUserRepository(client)
				user := &domain.User{Username: "existing", Password: string(hash), Status: domain.UserStatusActive}
				if err := repo.Create(ctx, user); err != nil {
					t.Fatal(err)
				}
				uc := &UserController{UserUsecase: usecase.NewUserUsecase(repo, 5*time.Second)}
				pc := &ProfileController{ProfileUsecase: usecase.NewProfileUsecase(repository.NewProfileRepository(client), 5*time.Second)}
				gin.SetMode(gin.TestMode)
				router := gin.New()
				router.Use(func(c *gin.Context) { c.Set(constants.UserID, user.ID); c.Next() })
				router.POST("/users", uc.CreateUser)
				router.PUT("/users/:id", uc.UpdateUser)
				router.PUT("/profile/password", pc.UpdatePassword)
				method, path := http.MethodPut, "/users/"+user.ID
				body := map[string]string{"password": test.password}
				wantStatus := http.StatusOK
				if operation == "create" {
					method, path, wantStatus = http.MethodPost, "/users", http.StatusCreated
					body["username"], body["email"] = "created", "created@example.com"
				} else if operation == "profile" {
					path = "/profile/password"
					body = map[string]string{"current_password": "old-password", "new_password": test.password}
				}
				unchanged := !test.valid
				if !test.valid && !(operation == "edit" && test.password == "") {
					wantStatus = http.StatusBadRequest
				}
				payload, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(method, path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				if w.Code != wantStatus {
					t.Fatalf("status = %d, body = %s; want %d", w.Code, w.Body, wantStatus)
				}
				stored, err := repo.GetByID(ctx, user.ID)
				if operation == "create" && test.valid {
					stored, err = repo.GetByIdentifier(ctx, "created")
				}
				if err != nil {
					t.Fatal(err)
				}
				if unchanged {
					if stored.Password != string(hash) {
						t.Fatal("invalid or omitted new password changed the stored hash")
					}
				} else if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(test.password)); err != nil {
					t.Fatalf("stored password does not match valid input: %v", err)
				}
			})
		}
	}
}
