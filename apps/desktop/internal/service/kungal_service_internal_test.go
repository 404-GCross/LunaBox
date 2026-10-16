package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type kungalRoundTripFunc func(*http.Request) (*http.Response, error)

func (f kungalRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func kungalJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestBuildKungalAuthURLUsesLoopbackPKCE(t *testing.T) {
	session, err := newKungalAuthSession()
	if err != nil {
		t.Fatalf("启动本地回调失败: %v", err)
	}
	defer session.shutdown()

	authURL, err := url.Parse(buildKungalAuthURL("public-client-id", session))
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	query := authURL.Query()
	if authURL.Scheme+"://"+authURL.Host+authURL.Path != kungalOAuthAuthorizeURL {
		t.Fatalf("授权端点不符合预期: %s", authURL.String())
	}
	redirectURI, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirectURI.Scheme != "http" || redirectURI.Hostname() != "127.0.0.1" || redirectURI.Port() == "" || redirectURI.Path != kungalOAuthCallbackPath {
		t.Fatalf("回调地址必须是带临时端口的 127.0.0.1 环回地址: %q", query.Get("redirect_uri"))
	}
	digest := sha256.Sum256([]byte(session.codeVerifier))
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(digest[:]) {
		t.Fatalf("PKCE 参数不符合预期: %s", authURL.RawQuery)
	}
	for _, scope := range []string{"openid", "profile", "catalog:read"} {
		if !strings.Contains(" "+query.Get("scope")+" ", " "+scope+" ") {
			t.Fatalf("授权请求缺少 scope %q: %q", scope, query.Get("scope"))
		}
	}
	if query.Get("state") != session.state || query.Has("client_secret") {
		t.Fatalf("授权请求防伪参数异常: %s", authURL.RawQuery)
	}
}

func TestMapGameStatusToKungalState(t *testing.T) {
	cases := map[enums.GameStatus]string{
		enums.StatusNotStarted: "wish",
		enums.StatusWantToPlay: "wish",
		enums.StatusPlaying:    "doing",
		enums.StatusCompleted:  "done",
		enums.StatusOnHold:     "on_hold",
		enums.StatusDropped:    "dropped",
	}
	for local, expected := range cases {
		if actual, ok := mapGameStatusToKungalState(local); !ok || actual != expected {
			t.Fatalf("状态 %s 应映射为 %s，实际 %q", local, expected, actual)
		}
	}
	if _, ok := mapGameStatusToKungalState("unknown"); ok {
		t.Fatalf("未知状态不应被映射")
	}
}

func TestKungalSyncGameChunkSkipsUnchangedAndReadsItemStatus(t *testing.T) {
	var posted []kungalWorkState
	service := NewKungalService()
	service.config = &appconf.AppConfig{
		KungalAccessToken:    "access-token",
		KungalRefreshToken:   "refresh-token",
		KungalTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	service.SetHTTPClient(&http.Client{Transport: kungalRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("请求缺少用户令牌")
		}
		switch {
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.String(), kungalWorkStatesURL):
			if req.URL.Query().Get("work_ids") != "1,2,3" {
				t.Fatalf("批量读取 work_ids 不符合预期: %q", req.URL.RawQuery)
			}
			return kungalJSONResponse(http.StatusOK, `{"object":"list","items":[{"object":"work_state","work_id":"1","state":"done","completion":"all"}]}`), nil
		case req.Method == http.MethodPost && req.URL.String() == kungalWorkStatesURL:
			var body struct {
				Items []kungalWorkState `json:"items"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("解析批量写请求失败: %v", err)
			}
			posted = body.Items
			return kungalJSONResponse(http.StatusMultiStatus, `{"items":[{"status":200,"object":"work_state"},{"status":404,"code":"NOT_FOUND","detail":"work not found."}]}`), nil
		default:
			t.Fatalf("意外请求: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})})

	failures := service.syncGameChunk(t.Context(), []models.Game{
		{ID: "g1", SourceID: "1", Status: enums.StatusCompleted},
		{ID: "g2", SourceID: "2", Status: enums.StatusPlaying},
		{ID: "g3", SourceID: "3", Status: enums.StatusDropped},
	})

	if len(posted) != 2 || posted[0].WorkID != "2" || posted[0].State != "doing" || posted[1].WorkID != "3" || posted[1].State != "dropped" {
		t.Fatalf("远端已一致的条目应跳过，其余按批写入: %+v", posted)
	}
	if posted[0].Completion != nil {
		t.Fatalf("状态写入不应伪造完成度")
	}
	if failures[0] != nil || failures[1] != nil {
		t.Fatalf("成功项不应报错: %v", failures)
	}
	if failures[2] == nil || !strings.Contains(failures[2].Error(), "404") {
		t.Fatalf("207 中失败的项应单独报错: %v", failures[2])
	}
}
