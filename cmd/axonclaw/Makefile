.PHONY: build clean package

BINARY_NAME=axonclaw
ZIP_NAME=axonclaw.zip

build:
	go build -o $(BINARY_NAME) .

package: build
	@COPYFILE_DISABLE=1 zip -r $(ZIP_NAME) $(BINARY_NAME) start.sh stop.sh -x "*.DS_Store" -x "__MACOSX/*"

clean:
	rm -f $(BINARY_NAME) $(ZIP_NAME)
