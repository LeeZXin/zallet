package zssh

import gossh "golang.org/x/crypto/ssh"

var (
	ciphers      = []string{"chacha20-poly1305@openssh.com", "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com", "aes256-gcm@openssh.com"}
	keyExchanges = []string{"curve25519-sha256", "ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521", "diffie-hellman-group14-sha256", "diffie-hellman-group14-sha1"}
	macs         = []string{"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256", "hmac-sha1"}
)

func NewCommonClientConfig(username string, signer gossh.Signer) *gossh.ClientConfig {
	return &gossh.ClientConfig{
		Config: gossh.Config{
			KeyExchanges: keyExchanges,
			Ciphers:      ciphers,
			MACs:         macs,
		},
		User: username,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}
}
