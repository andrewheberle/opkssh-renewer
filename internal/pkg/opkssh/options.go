package opkssh

import "log/slog"

type RenewerOption func(*Renewer)

func WithLogger(logger *slog.Logger) RenewerOption {
	return func(r *Renewer) {
		r.logger = logger
	}
}
