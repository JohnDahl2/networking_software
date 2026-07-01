module live-sniffer

go 1.26.2

require (
	github.com/google/gopacket v1.1.19
	github.com/jackc/pgx/v5 v5.9.2
	github.com/spf13/cobra v1.10.2
	networking_software/shared v0.0.0-00010101000000-000000000000
)

replace networking_software/shared => ../shared

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.0.0-20190412213103-97732733099d // indirect
	golang.org/x/text v0.36.0 // indirect
)
