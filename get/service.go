package get

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

// Ports of the on-demand services, as injected by service connections.
const (
	postgresPort = "5432"
	mySQLPort    = "3306"
)

// ServiceCmd is the shared base for the get sub-commands of on-demand services,
// like databases, key-value stores or search clusters.
//
// It has to be exported so that Kong initializes the embedded [format.Writer], see [format.Writer.BeforeApply].
type ServiceCmd struct {
	ResourceCmd
	PrintUser             bool `help:"Print the user. Requires name to be set." xor:"print" aliases:"print-database-user"`
	PrintPassword         bool `help:"Print the password. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string. Requires name to be set." xor:"print"`
	PrintCACert           bool `help:"Print the ca certificate. Requires name to be set." xor:"print"`
}

// service describes how the details of a specific on-demand service are printed by [ServiceCmd.run].
type service struct {
	kind string
	// connectionString returns a connection string for the resource,
	// using the user and password found in its connection secret.
	connectionString func(mg resource.Managed, user, password string) (string, error)
	caCert           func(resource.Managed) (string, error)
	printList        func(list resource.ManagedList, out *output, header bool) error
}

func (cmd *ServiceCmd) run(ctx context.Context, client *api.Client, out *output, list resource.ManagedList, svc service) error {
	if len(list.GetItems()) == 0 {
		return out.notFound(svc.kind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintUser {
		return cmd.printSecret(
			ctx,
			client,
			list.GetItems()[0],
			out,
			func(user, _ string) string { return user },
		)
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(
			ctx,
			client,
			list.GetItems()[0],
			out,
			func(_, pw string) string { return pw },
		)
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		mg := list.GetItems()[0]
		secrets, err := ConnectionSecretMap(ctx, client, mg)
		if err != nil {
			return err
		}

		user, pw, err := credentials(secrets)
		if err != nil {
			return fmt.Errorf("%s %s: %w", svc.kind, mg.GetName(), err)
		}

		str, err := svc.connectionString(mg, user, pw)
		if err != nil {
			return fmt.Errorf("%s %s: %w", svc.kind, mg.GetName(), err)
		}

		out.Println(str)
		return nil
	}

	if cmd.Name != "" && cmd.PrintCACert {
		ca, err := svc.caCert(list.GetItems()[0])
		if err != nil {
			return err
		}
		return WriteBase64(&out.Writer, ca)
	}

	switch out.Format {
	case full:
		return svc.printList(list, out, true)
	case noHeader:
		return svc.printList(list, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(
			list.GetItems(),
			format.PrintOpts{Out: &out.Writer},
		)
	case jsonOut:
		return format.PrettyPrintObjects(
			list.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			},
		)
	}

	return nil
}

// credentials returns the user and password of the connection secret of an on-demand service,
// which holds exactly one user mapped to its password.
func credentials(secrets map[string][]byte) (string, string, error) {
	if len(secrets) != 1 {
		return "", "", fmt.Errorf("expected exactly one user in connection secret, found %d", len(secrets))
	}

	for user, pw := range secrets {
		return user, string(pw), nil
	}

	return "", "", nil
}

// connectionURI builds a URI with embedded credentials.
// It follows the format of the DSNs injected into deplo.io applications by service connections,
// so that both can be used interchangeably.
func connectionURI(scheme, fqdn, port, user, password, path string, query url.Values) (string, error) {
	if fqdn == "" {
		return "", errors.New("no FQDN found, the service might not be ready yet")
	}

	u := &url.URL{
		Scheme:   scheme,
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(fqdn, port),
		Path:     path,
		RawQuery: query.Encode(),
	}

	return u.String(), nil
}

// postgresConnectionString according to the PostgreSQL documentation:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
// sslmode is require as there is no CA cert on disk to verify against.
func postgresConnectionString(fqdn, user, password, db string) (string, error) {
	return connectionURI("postgresql", fqdn, postgresPort, user, password, db, url.Values{"sslmode": {"require"}})
}

// mySQLConnectionString according to the MySQL documentation:
// https://dev.mysql.com/doc/refman/8.4/en/connecting-using-uri-or-key-value-pairs.html#connecting-using-uri
// ssl-mode is REQUIRED as there is no CA cert on disk to verify against.
func mySQLConnectionString(fqdn, user, password, db string) (string, error) {
	return connectionURI("mysql", fqdn, mySQLPort, user, password, db, url.Values{"ssl-mode": {"REQUIRED"}})
}
