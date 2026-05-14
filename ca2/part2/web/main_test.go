package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authpb "grpc/auth"
	filepb "grpc/file"

	"google.golang.org/grpc"
)

type mockAuthServer struct {
	authpb.UnimplementedAuthServiceServer
}

func (m *mockAuthServer) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	if req.GetUsername() == "admin" && req.GetPassword() == "admin123" {
		return &authpb.LoginResponse{Success: true}, nil
	}
	return &authpb.LoginResponse{Success: false}, nil
}

type mockFileServer struct {
	filepb.UnimplementedFileServiceServer
}

func (m *mockFileServer) DownloadFile(ctx context.Context, req *filepb.FileRequest) (*filepb.FileResponse, error) {
	if req.GetFilename() == "1.jpg" {
		return &filepb.FileResponse{
			Data:        []byte("fake_data"),
			ContentType: "image/jpeg",
		}, nil
	}
	return nil, grpc.ErrServerStopped
}

func setupMocks(t *testing.T) func() {
	authLis, _ := net.Listen("tcp", "127.0.0.1:0")
	authSrv := grpc.NewServer()
	authpb.RegisterAuthServiceServer(authSrv, &mockAuthServer{})
	go authSrv.Serve(authLis)

	fileLis, _ := net.Listen("tcp", "127.0.0.1:0")
	fileSrv := grpc.NewServer()
	filepb.RegisterFileServiceServer(fileSrv, &mockFileServer{})
	go fileSrv.Serve(fileLis)

	authServerAddr = authLis.Addr().String()
	fileServerAddr = fileLis.Addr().String()

	return func() {
		authSrv.Stop()
		fileSrv.Stop()
	}
}

func TestHandleLogin_Success(t *testing.T) {
	cleanup := setupMocks(t)
	defer cleanup()

	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin123")
	form.Add("filename", "1.jpg")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Login Successful") {
		t.Error("Expected success message not found in body")
	}
}

func TestHandleLogin_AuthFailure(t *testing.T) {
	cleanup := setupMocks(t)
	defer cleanup()

	form := url.Values{}
	form.Add("username", "baduser")
	form.Add("password", "badpass")
	form.Add("filename", "1.jpg")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handleLogin(w, req)

	if !strings.Contains(w.Body.String(), "Invalid username or password") {
		t.Error("Expected error message for invalid login not found")
	}
}