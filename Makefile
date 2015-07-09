XC_ARCH=386 amd64
XC_OS=linux darwin windows
version=`./envaws -v | sed 's/envaws version //g'`

install:
	go install

## GITHUB_TOKEN is needed
release: ./envaws
	rm -f pkg/*.exe pkg/*_amd64 pkg/*_386*
	ghr -u tmtk75 v$(version) pkg

./envaws:
	go build main.go

hash:
	shasum -a1 pkg/*_amd64.{gz,zip}

compress: pkg/envaws_win_amd64.zip pkg/envaws_darwin_amd64.gz pkg/envaws_linux_amd64.gz

pkg/envaws_win_amd64.zip pkg/envaws_darwin_amd64.gz pkg/envaws_linux_amd64.gz:
	gzip -k pkg/*_386
	gzip -k pkg/*_amd64
	for e in 386 amd64; do \
		mv pkg/envaws_windows_$$e.exe pkg/envaws_$$e.exe; \
		zip envaws_win_$$e.zip pkg/envaws_$$e.exe; \
		mv envaws_win_$$e.zip pkg; \
	done

build: clean
	gox \
	  -os="$(XC_OS)" \
	  -arch="$(XC_ARCH)" \
	  -output "pkg/{{.Dir}}_{{.OS}}_{{.Arch}}"

clean:
	rm -f pkg/*.gz pkg/*.zip

distclean:
	rm -rf envaws pkg

setup:
	go get -u github.com/mitchellh/gox
	gox -build-toolchain
