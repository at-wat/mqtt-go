test.crt: ca.crt test.csr test.key sans.conf Makefile
	openssl ca \
		-days 3650 \
		-in test.csr \
		-keyfile test.key \
		-cert ca.crt \
		-extfile sans.conf \
		-out $@
	openssl x509 -noout -text -in $@

test.csr: test.key Makefile
	openssl req \
		-new \
		-key test.key \
		-out $@ \
		-sha256 \
		-nodes \
	  -extensions v3_req \
		-addext subjectAltName=DNS:localhost \
		-subj "/C=JP/ST=Tokyo/L=Default City/O=Default Company Ltd/CN=localhost"
	openssl req -noout -text -in $@

ca.crt: ca.csr test.key Makefile
	openssl x509 \
		-days 3650 \
		-in ca.csr \
		-req \
		-signkey test.key \
		-extfile ca.conf \
		-out $@
	openssl x509 -noout -text -in $@

ca.csr: test.key Makefile
	openssl req \
		-new \
		-key test.key \
		-out $@ \
		-sha256 \
		-nodes \
		-extensions v3_req \
		-extensions v3_ca \
		-addext subjectAltName=DNS:localca \
		-addext basicConstraints=CA:true \
		-subj "/C=JP/ST=Tokyo/L=Default City/O=Default Company Ltd/CN=localca"
	openssl req -noout -text -in $@
