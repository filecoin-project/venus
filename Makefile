cborgen:
	cd venus-devtool && go run ./cborgen/*.go

test-venus-shared:
	cd venus-shared && go test -v ./...

test: test-venus-shared
