package skills

// LoadBuiltins returns embedded skills compiled into the binary.
// Currently empty — all skills are loaded from disk at runtime
// (user-level ~/.swifty/skills/ or project-level .swifty/skills/).
func LoadBuiltins() []*Skill {
	return nil
}
