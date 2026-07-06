# ==============================================================================
# Makefile helper functions for generate certificates files
#


# 可以直接make gen.ca.example生成特定组件example的证书，而不影响其他组件
.PHONY: gen.ca.server.%
gen.ca.server.%:
	$(eval Component := $(word 1,$(subst ., ,$*)))
	@echo "===========> Generating Server Certificate files for \"$(Component)\",Subjects:$(CERTIFICATES_SUBJECT),ALT_NAME:$(SERVER_CERTIFICATES_ALT_NAME)"
	@echo "===========> CERTIFICATE_DIR:$(CERTIFICATE_DIR)"
	@${ROOT_DIR}/scripts/gencerts.sh generate_server_certificate $(CERTIFICATE_DIR) $(Component)-server  $(CERTIFICATES_SUBJECT) $(SERVER_CERTIFICATES_ALT_NAME)

.PHONY: gen.ca.client.%
gen.ca.client.%:
	$(eval Component := $(word 1,$(subst ., ,$*)))
	@echo "===========> Generating Client Certificate files for \"$(Component)\",Subjects:$(CERTIFICATES_SUBJECT),ALT_NAME:$(CLIENT_CERTIFICATES_ALT_NAME)"
	@echo "===========> CERTIFICATE_DIR:$(CERTIFICATE_DIR)"
	@${ROOT_DIR}/scripts/gencerts.sh generate_client_certificate $(CERTIFICATE_DIR) $(Component)-client  $(CERTIFICATES_SUBJECT) $(CLIENT_CERTIFICATES_ALT_NAME)

# 生成组件的证书
# make CERTIFICATES=xxx gen.ca
# make gen.ca
.PHONY: gen.ca
gen.ca: $(addprefix gen.ca.server., $(CERTIFICATES)) $(addprefix gen.ca.client., $(CERTIFICATES))