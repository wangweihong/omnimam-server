# ==============================================================================
# Makefile helper functions for generate example files
#


## deecopy-gen-example: Run an example show how deepcopy auto generate api type's DeepCopy function.
.PHONY: deepcopy-gen-example
deepcopy-gen-example: tools.verify.deepcopy-gen
	@deepcopy-gen --input-dirs=./tools/deepcopy-gen/example --output-base=../

## code-gen-example: Run an example show how codegen auto generate error code .go definition file and .md file
code-gen-example: tools.verify.codegen
	@echo "===========> Generating error code go source files to path:${ROOT_DIR}/tools/codegen/example"
	@codegen -type=int ${ROOT_DIR}/tools/codegen/example
	@echo "===========> Generating error code markdown documentation to path:${ROOT_DIR}/tools/codegen/example/error_code_generated.md"
	@codegen -type=int -doc \
		-output ${ROOT_DIR}/tools/codegen/example/error_code_generated.md ${ROOT_DIR}/tools/codegen/example