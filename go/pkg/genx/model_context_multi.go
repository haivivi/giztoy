package genx

import "iter"

var _ ModelContext = (MultiModelContext)(nil)

type MultiModelContext []ModelContext

func ModelContexts(ctxs ...ModelContext) MultiModelContext {
	return MultiModelContext(ctxs)
}

func (mctx MultiModelContext) Prompts() iter.Seq[*Prompt] {
	return func(yield func(*Prompt) bool) {
		for _, ctx := range mctx {
			for prompt := range ctx.Prompts() {
				if !yield(prompt) {
					return
				}
			}
		}
	}
}

func (mctx MultiModelContext) Messages() iter.Seq[*Message] {
	return func(yield func(*Message) bool) {
		for _, ctx := range mctx {
			for message := range ctx.Messages() {
				if !yield(message) {
					return
				}
			}
		}
	}
}

func (mctx MultiModelContext) CoTs() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, ctx := range mctx {
			for cot := range ctx.CoTs() {
				if !yield(cot) {
					return
				}
			}
		}
	}
}

func (mctx MultiModelContext) Tools() iter.Seq[Tool] {
	return func(yield func(Tool) bool) {
		for _, ctx := range mctx {
			for tool := range ctx.Tools() {
				if !yield(tool) {
					return
				}
			}
		}
	}
}

func (mctx MultiModelContext) Params() *ModelParams {
	for _, ctx := range mctx {
		if ctx.Params() != nil {
			return ctx.Params()
		}
	}
	return nil
}
