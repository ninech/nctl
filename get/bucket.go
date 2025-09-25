package get

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bucketCmd struct {
	resourceCmd
	PrintPermissions       bool `help:"Print the Bucket's permission grants." xor:"print"`
	PrintLifecyclePolicies bool `help:"Print the Bucket's lifecycle policies." xor:"print"`
	PrintCORS              bool `help:"Print the Bucket's CORS config." xor:"print"`
	PrintCustomHostnames   bool `help:"Print the Bucket's custom hostnames." xor:"print"`
}

const (
	colEndpoint        = "ENDPOINT"
	colPublicURL       = "PUBLIC URL"
	colBytesUsed       = "BYTES USED"
	colObjectCount     = "OBJECT COUNT"
	colDNSVerification = "CUSTOM HOSTNAMES VERIFICATION"
)

func (cmd *bucketCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *bucketCmd) list() client.ObjectList {
	return &storage.BucketList{}
}

func (cmd *bucketCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	bucketList := list.(*storage.BucketList)
	if len(bucketList.Items) == 0 {
		return out.printEmptyMessage(storage.BucketKind, client.Project)
	}
	bucket := &bucketList.Items[0]

	if cmd.printFlagSet() {
		if cmd.Name == "" {
			return fmt.Errorf("name needs to be set to print bucket information")
		}

		if cmd.PrintPermissions {
			return printBucketPermissions(bucket, out)
		}
		if cmd.PrintLifecyclePolicies {
			return printBucketLifecyclePolicies(bucket, out)
		}
		if cmd.PrintCORS {
			return printBucketCORS(bucket, out)
		}
		if cmd.PrintCustomHostnames {
			return printBucketCustomHostnames(bucket, out)
		}
	}

	switch out.Format {
	case full:
		return printBucket(bucketList.Items, out, true)
	case noHeader:
		return printBucket(bucketList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(bucketList.GetItems(), format.PrintOpts{Out: out.writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			bucketList.GetItems(),
			format.PrintOpts{
				Out:    out.writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	case stats:
		return cmd.printStats(bucketList.Items, out)
	}

	return nil
}

func (cmd *bucketCmd) printFlagSet() bool {
	return cmd.PrintPermissions || cmd.PrintLifecyclePolicies || cmd.PrintCORS || cmd.PrintCustomHostnames
}

func (cmd *bucketCmd) Help() string {
	return "To get an overview of the bucket usage, use the flag '-o stats':\n" +
		"\t" + colEndpoint + ": API endpoint to use with S3 compatible clients.\n" +
		"\t" + colPublicURL + ": PublicURL where the bucket content is accessible if set to PublicRead.\n" +
		"\t" + colBytesUsed + ": The amount of bytes a bucket is currently using.\n" +
		"\t" + colObjectCount + ": The number of objects a bucket has.\n" +
		"\t" + colDNSVerification + ": Summary of DNS verification for all custom hostnames (use '--print-custom-hostnames' for details).\n"
}

func printBucket(buckets []storage.Bucket, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "LOCATION", "PUBLIC READ", "PUBLIC LIST", "VERSIONING")
	}

	for _, b := range buckets {
		fp := b.Spec.ForProvider
		out.writeTabRow(
			b.Namespace, b.Name,
			string(fp.Location),
			fmt.Sprintf("%t", fp.PublicRead),
			fmt.Sprintf("%t", fp.PublicList),
			fmt.Sprintf("%t", fp.Versioning),
		)
	}

	return out.tabWriter.Flush()
}

func (cmd *bucketCmd) printStats(buckets []storage.Bucket, out *output) error {
	out.writeHeader("NAME", colEndpoint, colPublicURL, colBytesUsed, colObjectCount, colDNSVerification)
	for _, b := range buckets {
		ap := b.Status.AtProvider
		out.writeTabRow(
			b.Namespace,
			b.Name,
			dashIfEmpty(ap.Endpoint),
			dashIfEmpty(ap.PublicURL),
			itoa64(ap.BytesUsed),
			itoa64(ap.ObjectCount),
			dnsVerificationSummary(b.Spec.ForProvider.CustomHostnames, ap.CustomHostnamesVerification))
	}

	return out.tabWriter.Flush()
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func printBucketPermissions(b *storage.Bucket, out *output) error {
	perms := b.Spec.ForProvider.Permissions
	if len(perms) == 0 {
		fmt.Fprintf(out.writer, "No permissions defined for bucket %q\n", b.Name)
		return nil
	}

	if out.Format == full {
		out.writeHeader("NAME", "ROLE", "USERS")
	}
	for _, p := range perms {
		var users []string
		for _, ref := range p.BucketUserRefs {
			if ref != nil && ref.Name != "" {
				users = append(users, ref.Name)
			}
		}
		out.writeTabRow(
			b.Namespace, b.Name,
			string(p.Role),
			joinOrDash(users),
		)
	}
	return out.tabWriter.Flush()
}

func printBucketLifecyclePolicies(b *storage.Bucket, out *output) error {
	rules := b.Spec.ForProvider.LifecyclePolicies
	if len(rules) == 0 {
		fmt.Fprintf(out.writer, "No lifecycle policies defined for bucket %q\n", b.Name)
		return nil
	}

	if out.Format == full {
		out.writeHeader("NAME", "PREFIX", "EXPIRE AFTER", "IS LIVE")
	}
	for _, r := range rules {
		exp := "-"
		if r.ExpireAfter.Duration != 0 {
			exp = r.ExpireAfter.Duration.String()
		} else if r.ExpireAfterDays != 0 {
			exp = fmt.Sprintf("%dd", r.ExpireAfterDays)
		}
		out.writeTabRow(
			b.Namespace, b.Name,
			dashIfEmpty(r.Prefix),
			exp,
			strconv.FormatBool(r.IsLive),
		)
	}
	return out.tabWriter.Flush()
}

func printBucketCORS(b *storage.Bucket, out *output) error {
	cfg := b.Spec.ForProvider.CORS
	if cfg == nil {
		fmt.Fprintf(out.writer, "No CORS configuration defined for bucket %q\n", b.Name)
		return nil
	}

	if out.Format == full {
		out.writeHeader("NAME", "ORIGINS", "RESPONSE HEADERS", "MAX-AGE (s)")
	}
	if cfg == nil {
		return out.tabWriter.Flush()
	}
	out.writeTabRow(
		b.Namespace, b.Name,
		joinOrDash(cfg.Origins),
		joinOrDash(cfg.ResponseHeaders),
		fmt.Sprintf("%d", cfg.MaxAge),
	)
	return out.tabWriter.Flush()
}

func printBucketCustomHostnames(b *storage.Bucket, out *output) error {
	hosts := b.Spec.ForProvider.CustomHostnames
	if len(hosts) == 0 {
		fmt.Fprintf(out.writer, "No custom hostnames defined for bucket %q\n", b.Name)
		return nil
	}

	st := b.Status.AtProvider.CustomHostnamesVerification
	if out.Format == full {
		out.writeHeader("NAME", "HOSTNAME", "CHECK TYPE", "EXPECTED", "VERIFIED", "LAST SUCCESS", "ERROR")
	}

	for _, host := range hosts {
		for _, ct := range []meta.DNSCheckType{
			meta.DNSCheckCNAME,
			meta.DNSCheckTXT,
			meta.DNSCheckCAA,
		} {
			entry, ok := st.StatusEntries.CheckTypeEntry(host, ct)
			if !ok {
				continue
			}

			verified := entry.Verified()
			last := "-"
			if entry.LatestSuccess != nil {
				last = entry.LatestSuccess.Format(time.RFC3339)
			}
			errMsg := "-"
			if entry.Error != nil {
				errMsg = entry.Error.Message
			}

			expected := "-"
			switch ct {
			case meta.DNSCheckCNAME:
				expected = dashIfEmpty(st.CNAMETarget)
			case meta.DNSCheckTXT:
				expected = dashIfEmpty(st.TXTRecordValue)
			case meta.DNSCheckCAA:
				expected = "CAA record required"
			}

			out.writeTabRow(
				b.Namespace, b.Name,
				host,
				ct.String(),
				expected,
				fmt.Sprintf("%t", verified),
				last,
				errMsg,
			)
		}

		// If a host has no status entries at all yet, emit a single pending row.
		if !hostHasAnyEntry(st.StatusEntries, host) {
			out.writeTabRow(host, "-", "-", "false", "-", "pending")
		}
	}

	return out.tabWriter.Flush()
}

func hostHasAnyEntry(vsl meta.DNSVerificationStatusEntries, host string) bool {
	for _, ct := range []meta.DNSCheckType{
		meta.DNSCheckCNAME,
		meta.DNSCheckTXT,
		meta.DNSCheckCAA,
	} {
		if _, ok := vsl.CheckTypeEntry(host, ct); ok {
			return true
		}
	}
	return false
}
func joinOrDash(ss []string) string {
	if len(ss) == 0 {
		return "-"
	}
	return strings.Join(ss, ",")
}
func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// Summarize verification as "VERIFIED/TOTAL (+N failed)" or "N/TOTAL (pending)"
func dnsVerificationSummary(hosts []string, st meta.DNSVerificationStatus) string {
	total := len(hosts)
	if total == 0 {
		return "-"
	}

	verifiedSet := make(map[string]struct{})
	for _, h := range st.StatusEntries.VerifiedHosts() {
		verifiedSet[h] = struct{}{}
	}

	verified, failed := 0, 0
	for _, host := range hosts {
		if _, ok := verifiedSet[host]; ok {
			verified++
			continue
		}
		if hostHasAnyError(st.StatusEntries, host) {
			failed++
			continue
		}
		// else pending
	}

	pending := total - verified - failed
	switch {
	case verified == 0 && failed == 0:
		return fmt.Sprintf("%d/%d (pending)", pending, total)
	case failed > 0:
		return fmt.Sprintf("%d/%d (+%d failed)", verified, total, failed)
	default:
		return fmt.Sprintf("%d/%d", verified, total)
	}
}

// Any error recorded for the host across any check type?
func hostHasAnyError(vsl meta.DNSVerificationStatusEntries, host string) bool {
	for _, ct := range []meta.DNSCheckType{
		meta.DNSCheckCNAME,
		meta.DNSCheckTXT,
		meta.DNSCheckCAA,
	} {
		if e, ok := vsl.CheckTypeEntry(host, ct); ok && e.Error != nil {
			return true
		}
	}
	return false
}
