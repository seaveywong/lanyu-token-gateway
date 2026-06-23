module github.com/seaveywong/lanyu-token-gateway/packages/auth

go 1.24

require (
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.37.0
)

require golang.org/x/sys v0.32.0 // indirect

replace (
	golang.org/x/crypto v0.37.0 => github.com/golang/crypto v0.37.0
	golang.org/x/sys v0.32.0 => github.com/golang/sys v0.32.0
)
