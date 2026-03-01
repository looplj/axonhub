.PHONY: build clean package

BINARY_NAME=axonclaw
ZIP_NAME=axonclaw.zip

build:
	go build -o $(BINARY_NAME) .

package: build
	@COPYFILE_DISABLE=1 zip -r $(ZIP_NAME) $(BINARY_NAME) *.sh *.bat *.ps1 -x "*.DS_Store" -x "__MACOSX/*"

clean:
	rm -f $(BINARY_NAME) $(ZIP_NAME)
