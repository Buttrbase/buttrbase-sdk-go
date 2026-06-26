package buttrbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// ---- Wallet ----

func TestWallet_HappyPath(t *testing.T) {
	budget := int64(10000)
	period := "monthly"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/wallet")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": WalletSummary{
				BalanceCents:     5000,
				BudgetLimitCents: &budget,
				BudgetPeriod:     &period,
			},
		})
	})
	res, err := c.Wallet(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.BalanceCents != 5000 {
		t.Errorf("expected balance_cents=5000, got %d", res.BalanceCents)
	}
	if res.BudgetLimitCents == nil || *res.BudgetLimitCents != 10000 {
		t.Errorf("expected budget_limit_cents=10000, got %v", res.BudgetLimitCents)
	}
	if res.BudgetPeriod == nil || *res.BudgetPeriod != "monthly" {
		t.Errorf("expected budget_period=monthly, got %v", res.BudgetPeriod)
	}
}

func TestWallet_NoBudget(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{
				"balance_cents":     int64(100),
				"budget_limit_cents": nil,
				"budget_period":      nil,
			},
		})
	})
	res, err := c.Wallet(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.BalanceCents != 100 {
		t.Errorf("expected balance_cents=100, got %d", res.BalanceCents)
	}
	if res.BudgetLimitCents != nil {
		t.Errorf("expected nil budget_limit_cents, got %v", res.BudgetLimitCents)
	}
}

func TestWallet_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.Wallet(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- WalletTransactions ----

func TestWalletTransactions_HappyPath(t *testing.T) {
	desc := "Top-up"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/wallet/transactions")
		assertAuth(t, r)
		// Check pagination params
		q := r.URL.Query()
		if q.Get("limit") != "10" {
			t.Errorf("expected limit=10, got %q", q.Get("limit"))
		}
		if q.Get("offset") != "20" {
			t.Errorf("expected offset=20, got %q", q.Get("offset"))
		}
		writeJSON(w, 200, map[string]any{
			"data": []WalletTransaction{
				{ID: 1, Kind: "deposit", AmountCents: 1000, Description: &desc, CreatedAt: "2026-01-01"},
				{ID: 2, Kind: "withdrawal", AmountCents: -500, CreatedAt: "2026-01-02"},
			},
		})
	})
	res, err := c.WalletTransactions(context.Background(), 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(res))
	}
	if res[0].Kind != "deposit" || res[0].AmountCents != 1000 {
		t.Errorf("unexpected first transaction: %+v", res[0])
	}
	if res[1].Kind != "withdrawal" {
		t.Errorf("expected withdrawal, got %q", res[1].Kind)
	}
}

func TestWalletTransactions_Empty(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"data": []WalletTransaction{}})
	})
	res, err := c.WalletTransactions(context.Background(), 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty slice, got %d items", len(res))
	}
}

func TestWalletTransactions_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.WalletTransactions(context.Background(), 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- Subscriptions ----

func TestSubscriptions_HappyPath(t *testing.T) {
	userUUID := "user-uuid-1"
	priceID := 5
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/subscriptions")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []SubscriptionItem{
				{
					ID:                     1,
					UserUUID:               &userUUID,
					PriceID:                &priceID,
					Provider:               "stripe",
					ProviderSubscriptionID: "sub_abc123",
					Status:                 "active",
					CreatedAt:              "2026-01-01",
					UpdatedAt:              "2026-01-01",
				},
			},
		})
	})
	res, err := c.Subscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(res))
	}
	if res[0].Provider != "stripe" {
		t.Errorf("expected provider=stripe, got %q", res[0].Provider)
	}
	if res[0].Status != "active" {
		t.Errorf("expected status=active, got %q", res[0].Status)
	}
}

func TestSubscriptions_Empty(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"data": []SubscriptionItem{}})
	})
	res, err := c.Subscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 subscriptions, got %d", len(res))
	}
}

func TestSubscriptions_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.Subscriptions(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CreateSubscription ----

func TestCreateSubscription_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/subscriptions")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["price_id"] == nil {
			t.Error("expected price_id in body")
		}
		writeJSON(w, 201, map[string]any{
			"data": SubscriptionItem{
				ID:                     2,
				Provider:               "stripe",
				ProviderSubscriptionID: "sub_new123",
				Status:                 "active",
				CreatedAt:              "2026-01-01",
				UpdatedAt:              "2026-01-01",
			},
		})
	})
	req := CreateSubscriptionRequest{PriceID: 42}
	res, err := c.CreateSubscription(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != 2 {
		t.Errorf("expected id=2, got %d", res.ID)
	}
	if res.Status != "active" {
		t.Errorf("expected status=active, got %q", res.Status)
	}
}

func TestCreateSubscription_WithQuantity(t *testing.T) {
	qty := 3
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["quantity"] == nil {
			t.Error("expected quantity in body")
		}
		writeJSON(w, 201, map[string]any{
			"data": SubscriptionItem{ID: 3, Provider: "stripe", ProviderSubscriptionID: "s", Status: "active", CreatedAt: "x", UpdatedAt: "x"},
		})
	})
	req := CreateSubscriptionRequest{PriceID: 10, Quantity: &qty}
	_, err := c.CreateSubscription(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSubscription_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{"detail": "invalid price_id"})
	})
	_, err := c.CreateSubscription(context.Background(), CreateSubscriptionRequest{PriceID: -1})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CancelSubscription ----

func TestCancelSubscription_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodDelete)
		assertPath(t, r, "/api/subscriptions/42")
		assertAuth(t, r)
		w.WriteHeader(204)
	})
	err := c.CancelSubscription(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCancelSubscription_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "subscription not found"})
	})
	err := c.CancelSubscription(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ReportUsage ----

func TestReportUsage_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/usage/report")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["metric"] == nil {
			t.Error("expected metric in body")
		}
		if body["quantity"] == nil {
			t.Error("expected quantity in body")
		}
		w.WriteHeader(204)
	})
	event := UsageEvent{Metric: "api_calls", Quantity: 1.0}
	err := c.ReportUsage(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReportUsage_WithOptionalFields(t *testing.T) {
	orgUUID := "org-uuid-1"
	appUUID := "app-uuid-1"
	ts := "2026-01-01T00:00:00Z"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["org_uuid"] == nil {
			t.Error("expected org_uuid in body")
		}
		if body["app_uuid"] == nil {
			t.Error("expected app_uuid in body")
		}
		if body["timestamp"] == nil {
			t.Error("expected timestamp in body")
		}
		w.WriteHeader(204)
	})
	event := UsageEvent{
		Metric:    "storage_gb",
		Quantity:  2.5,
		OrgUUID:   &orgUUID,
		AppUUID:   &appUUID,
		Timestamp: &ts,
	}
	err := c.ReportUsage(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReportUsage_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "invalid metric"})
	})
	err := c.ReportUsage(context.Background(), UsageEvent{Metric: "", Quantity: 0})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- OrgTeams ----

func TestOrgTeams_HappyPath(t *testing.T) {
	desc := "Engineering team"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/organizations/org-abc/teams")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []TeamItem{
				{ID: 1, TeamUUID: "team-uuid-1", OrgUUID: "org-abc", Name: "Engineering", Description: &desc},
				{ID: 2, TeamUUID: "team-uuid-2", OrgUUID: "org-abc", Name: "Design"},
			},
		})
	})
	res, err := c.OrgTeams(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(res))
	}
	if res[0].Name != "Engineering" {
		t.Errorf("expected name=Engineering, got %q", res[0].Name)
	}
	if res[0].Description == nil || *res[0].Description != "Engineering team" {
		t.Errorf("expected description='Engineering team', got %v", res[0].Description)
	}
	if res[1].Description != nil {
		t.Errorf("expected nil description for Design team, got %v", res[1].Description)
	}
}

func TestOrgTeams_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.OrgTeams(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- UserTeams ----

func TestUserTeams_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/users/user-xyz/teams")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []TeamItem{
				{ID: 3, TeamUUID: "team-uuid-3", OrgUUID: "org-def", Name: "Backend"},
			},
		})
	})
	res, err := c.UserTeams(context.Background(), "user-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 team, got %d", len(res))
	}
	if res[0].Name != "Backend" {
		t.Errorf("expected name=Backend, got %q", res[0].Name)
	}
}

func TestUserTeams_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "user not found"})
	})
	_, err := c.UserTeams(context.Background(), "bad-user")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- MyApps ----

func TestMyApps_HappyPath(t *testing.T) {
	role := "admin"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/me/apps")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []AppEntry{
				{AppUUID: "app-uuid-1", AppName: "MySaaS", Role: &role},
			},
		})
	})
	res, err := c.MyApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 app, got %d", len(res))
	}
	if res[0].AppName != "MySaaS" {
		t.Errorf("expected AppName=MySaaS, got %q", res[0].AppName)
	}
	if res[0].Role == nil || *res[0].Role != "admin" {
		t.Errorf("expected role=admin, got %v", res[0].Role)
	}
}

func TestMyApps_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.MyApps(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- AppOrgs ----

func TestAppOrgs_HappyPath(t *testing.T) {
	role := "owner"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/apps/app-uuid-1/organizations")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []OrgEntry{
				{OrgUUID: "org-uuid-1", OrgName: "Acme Inc", Role: &role},
			},
		})
	})
	res, err := c.AppOrgs(context.Background(), "app-uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 org, got %d", len(res))
	}
	if res[0].OrgName != "Acme Inc" {
		t.Errorf("expected OrgName='Acme Inc', got %q", res[0].OrgName)
	}
}

func TestAppOrgs_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "app not found"})
	})
	_, err := c.AppOrgs(context.Background(), "bad-app")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- AppCredentials ----

func TestAppCredentials_HappyPath(t *testing.T) {
	prefix := "bb_live_sk_abc"
	createdAt := "2026-01-01"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/apps/app-uuid-1/credentials")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": AppCredentialsResponse{
				AppName:        "MySaaS",
				SandboxEnabled: true,
				Live: &AppCredentialInfo{
					Environment:        "live",
					ClientID:           "bb_live_cid_abc",
					ClientSecretPrefix: &prefix,
					IsActive:           true,
					CreatedAt:          &createdAt,
				},
			},
		})
	})
	res, err := c.AppCredentials(context.Background(), "app-uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.AppName != "MySaaS" {
		t.Errorf("expected AppName=MySaaS, got %q", res.AppName)
	}
	if !res.SandboxEnabled {
		t.Error("expected sandbox_enabled=true")
	}
	if res.Live == nil {
		t.Fatal("expected Live credentials")
	}
	if res.Live.ClientID != "bb_live_cid_abc" {
		t.Errorf("expected ClientID=bb_live_cid_abc, got %q", res.Live.ClientID)
	}
}

func TestAppCredentials_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "admin only"})
	})
	_, err := c.AppCredentials(context.Background(), "app-uuid-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- EnableSandbox ----

func TestEnableSandbox_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPatch)
		assertPath(t, r, "/api/apps/app-uuid-1")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["sandbox_enabled"] != true {
			t.Errorf("expected sandbox_enabled=true, got %v", body["sandbox_enabled"])
		}
		w.WriteHeader(204)
	})
	err := c.EnableSandbox(context.Background(), "app-uuid-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnableSandbox_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "admin only"})
	})
	err := c.EnableSandbox(context.Background(), "app-uuid-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RotateCredentials ----

func TestRotateCredentials_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/apps/app-uuid-1/credentials/live/rotate")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{"client_id": "bb_live_cid_new", "rotated": true})
	})
	res, err := c.RotateCredentials(context.Background(), "app-uuid-1", "live")
	if err != nil {
		t.Fatal(err)
	}
	if res["rotated"] != true {
		t.Errorf("expected rotated=true, got %v", res["rotated"])
	}
}

func TestRotateCredentials_Sandbox(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertPath(t, r, "/api/apps/app-uuid-1/credentials/sandbox/rotate")
		writeJSON(w, 200, map[string]any{"rotated": true})
	})
	_, err := c.RotateCredentials(context.Background(), "app-uuid-1", "sandbox")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRotateCredentials_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "app not found"})
	})
	_, err := c.RotateCredentials(context.Background(), "bad-app", "live")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RefreshToken ----

func TestRefreshToken_HappyPath(t *testing.T) {
	refreshTok := "new-refresh-token"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/app/auth/refresh")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["refresh"] != "old-refresh-tok" {
			t.Errorf("expected refresh=old-refresh-tok, got %v", body["refresh"])
		}
		writeJSON(w, 200, AccessToken{Token: "new-access-token", RefreshToken: &refreshTok})
	})
	res, err := c.RefreshToken(context.Background(), "old-refresh-tok")
	if err != nil {
		t.Fatal(err)
	}
	if res.Token != "new-access-token" {
		t.Errorf("expected token=new-access-token, got %q", res.Token)
	}
	if res.RefreshToken == nil || *res.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh_token=new-refresh-token, got %v", res.RefreshToken)
	}
	// Verify the client's AccessToken is updated
	if c.AccessToken != "new-access-token" {
		t.Errorf("expected client AccessToken to be updated, got %q", c.AccessToken)
	}
}

func TestRefreshToken_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "refresh token expired"})
	})
	_, err := c.RefreshToken(context.Background(), "expired-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PricingPreviewTyped ----

func TestPricingPreviewTyped_HappyPath(t *testing.T) {
	coupon := "SAVE10"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/pricing/preview")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["price_id"] == nil {
			t.Error("expected price_id in body")
		}
		if body["coupon_code"] == nil {
			t.Error("expected coupon_code in body")
		}
		discount := int64(100)
		tax := int64(50)
		region := "us-east"
		writeJSON(w, 200, map[string]any{
			"data": PricingPreviewResponse{
				AmountCents:    999,
				Currency:       "USD",
				DiscountCents:  &discount,
				TaxCents:       &tax,
				FinalCents:     949,
				RegionResolved: &region,
			},
		})
	})
	req := PricingPreviewRequest{PriceID: 1, CouponCode: &coupon}
	res, err := c.PricingPreviewTyped(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountCents != 999 {
		t.Errorf("expected amount_cents=999, got %d", res.AmountCents)
	}
	if res.Currency != "USD" {
		t.Errorf("expected currency=USD, got %q", res.Currency)
	}
	if res.FinalCents != 949 {
		t.Errorf("expected final_cents=949, got %d", res.FinalCents)
	}
}

func TestPricingPreviewTyped_Minimal(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// coupon_code should not be present when nil
		if body["coupon_code"] != nil {
			t.Errorf("expected no coupon_code, got %v", body["coupon_code"])
		}
		writeJSON(w, 200, map[string]any{
			"data": PricingPreviewResponse{AmountCents: 500, Currency: "USD", FinalCents: 500},
		})
	})
	req := PricingPreviewRequest{PriceID: 2}
	res, err := c.PricingPreviewTyped(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountCents != 500 {
		t.Errorf("expected 500, got %d", res.AmountCents)
	}
}

func TestPricingPreviewTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "price not found"})
	})
	_, err := c.PricingPreviewTyped(context.Background(), PricingPreviewRequest{PriceID: 9999})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PricingQuoteTyped ----

func TestPricingQuoteTyped_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/pricing/quote")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{"quote_id": "q-123", "price_id": 1, "expires_at": "2026-01-01T01:00:00Z"},
		})
	})
	res, err := c.PricingQuoteTyped(context.Background(), PricingPreviewRequest{PriceID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res["quote_id"] != "q-123" {
		t.Errorf("expected quote_id=q-123, got %v", res["quote_id"])
	}
}

func TestPricingQuoteTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "price not found"})
	})
	_, err := c.PricingQuoteTyped(context.Background(), PricingPreviewRequest{PriceID: 9999})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CheckoutSessionTyped ----

func TestCheckoutSessionTyped_HappyPath(t *testing.T) {
	quoteID := "q-123"
	sessionID := "sess_abc"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/pricing/checkout-session")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["price_id"] == nil {
			t.Error("expected price_id in body")
		}
		writeJSON(w, 200, map[string]any{
			"data": CheckoutSessionResponse{
				PaymentURL: "https://pay.stripe.com/cs/test",
				SessionID:  &sessionID,
				Provider:   "stripe",
			},
		})
	})
	req := CheckoutSessionTypedRequest{PriceID: 1, QuoteID: &quoteID}
	res, err := c.CheckoutSessionTyped(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.PaymentURL == "" {
		t.Error("expected non-empty payment_url")
	}
	if res.Provider != "stripe" {
		t.Errorf("expected provider=stripe, got %q", res.Provider)
	}
	if res.SessionID == nil || *res.SessionID != "sess_abc" {
		t.Errorf("expected session_id=sess_abc, got %v", res.SessionID)
	}
}

func TestCheckoutSessionTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{"detail": "sandbox not allowed"})
	})
	_, err := c.CheckoutSessionTyped(context.Background(), CheckoutSessionTypedRequest{PriceID: 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CheckEntitlementTyped ----

func TestCheckEntitlementTyped_Granted(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/entitlements/check")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["feature_key"] != "advanced_analytics" {
			t.Errorf("expected feature_key=advanced_analytics, got %v", body["feature_key"])
		}
		writeJSON(w, 200, map[string]any{
			"data": EntitlementResult{Granted: true},
		})
	})
	res, err := c.CheckEntitlementTyped(context.Background(), "advanced_analytics")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Granted {
		t.Error("expected granted=true")
	}
}

func TestCheckEntitlementTyped_Denied(t *testing.T) {
	reason := "plan_limit"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"data": EntitlementResult{Granted: false, Reason: &reason},
		})
	})
	res, err := c.CheckEntitlementTyped(context.Background(), "premium_feature")
	if err != nil {
		t.Fatal(err)
	}
	if res.Granted {
		t.Error("expected granted=false")
	}
	if res.Reason == nil || *res.Reason != "plan_limit" {
		t.Errorf("expected reason=plan_limit, got %v", res.Reason)
	}
}

func TestCheckEntitlementTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.CheckEntitlementTyped(context.Background(), "some_feature")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- BatchCheckEntitlementsTyped ----

func TestBatchCheckEntitlementsTyped_HappyPath(t *testing.T) {
	reason := "plan_limit"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/entitlements/check/batch")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["feature_keys"] == nil {
			t.Error("expected feature_keys in body")
		}
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{
				"feature_a": map[string]any{"granted": true},
				"feature_b": map[string]any{"granted": false, "reason": "plan_limit"},
			},
		})
	})
	res, err := c.BatchCheckEntitlementsTyped(context.Background(), []string{"feature_a", "feature_b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if !res["feature_a"].Granted {
		t.Error("expected feature_a granted=true")
	}
	if res["feature_b"].Granted {
		t.Error("expected feature_b granted=false")
	}
	_ = reason // used in server handler
}

func TestBatchCheckEntitlementsTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.BatchCheckEntitlementsTyped(context.Background(), []string{"feature_a"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetEffectiveEntitlementsTyped ----

func TestGetEffectiveEntitlementsTyped_HappyPath(t *testing.T) {
	reason := "pro_plan"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/entitlements/effective")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []EffectiveEntitlement{
				{FeatureKey: "feature_a", Granted: true, Reason: &reason},
				{FeatureKey: "feature_b", Granted: false},
			},
		})
	})
	res, err := c.GetEffectiveEntitlementsTyped(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 entitlements, got %d", len(res))
	}
	if res[0].FeatureKey != "feature_a" || !res[0].Granted {
		t.Errorf("unexpected first entitlement: %+v", res[0])
	}
	if res[1].FeatureKey != "feature_b" || res[1].Granted {
		t.Errorf("unexpected second entitlement: %+v", res[1])
	}
}

func TestGetEffectiveEntitlementsTyped_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.GetEffectiveEntitlementsTyped(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- JSON serialization round-trips ----

func TestUsageEvent_JSONOmitsNilFields(t *testing.T) {
	event := UsageEvent{Metric: "api_calls", Quantity: 1.0}
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["org_uuid"]; ok {
		t.Error("expected org_uuid to be omitted when nil")
	}
	if _, ok := m["app_uuid"]; ok {
		t.Error("expected app_uuid to be omitted when nil")
	}
	if _, ok := m["timestamp"]; ok {
		t.Error("expected timestamp to be omitted when nil")
	}
}

func TestCreateSubscriptionRequest_JSONOmitsNilQuantity(t *testing.T) {
	req := CreateSubscriptionRequest{PriceID: 5}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["quantity"]; ok {
		t.Error("expected quantity to be omitted when nil")
	}
}

func TestPricingPreviewRequest_JSONOmitsNilFields(t *testing.T) {
	req := PricingPreviewRequest{PriceID: 3}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["coupon_code"]; ok {
		t.Error("expected coupon_code to be omitted when nil")
	}
	if _, ok := m["seats"]; ok {
		t.Error("expected seats to be omitted when nil")
	}
	if _, ok := m["country"]; ok {
		t.Error("expected country to be omitted when nil")
	}
}

// Ensure the path is built correctly for WalletTransactions with zero offset.
func TestWalletTransactions_ZeroOffset(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expected := fmt.Sprintf("/api/wallet/transactions?limit=50&offset=0")
		if r.URL.Path+"?"+r.URL.RawQuery != expected {
			t.Errorf("expected path+query=%q, got %q", expected, r.URL.Path+"?"+r.URL.RawQuery)
		}
		writeJSON(w, 200, map[string]any{"data": []WalletTransaction{}})
	})
	_, err := c.WalletTransactions(context.Background(), 50, 0)
	if err != nil {
		t.Fatal(err)
	}
}
