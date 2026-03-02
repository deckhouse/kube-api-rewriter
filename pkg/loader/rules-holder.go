package loader

import "github.com/deckhouse/kube-api-rewriter/pkg/rewriter"

type RulesHolder struct {
	rewriteRules *rewriter.RewriteRules
}

var holder *RulesHolder

var UseRules = func(rules *rewriter.RewriteRules) {
	if holder == nil {
		holder = &RulesHolder{}
	}
	holder.rewriteRules = rules
}

var GetRules = func() *rewriter.RewriteRules {
	return holder.rewriteRules
}
