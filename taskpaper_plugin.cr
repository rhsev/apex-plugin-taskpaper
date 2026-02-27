require "json"

payload = JSON.parse(STDIN.gets_to_end)
text    = payload["text"].as_s

# Handle YAML frontmatter — convert to bold key: value display
if text.starts_with?("---\n")
  fm_end_pos = text.index("\n---\n", 4) || text.index("\n...\n", 4)
  if fm_end_pos
    fm_content = text[4...fm_end_pos]
    rest = text[(fm_end_pos + 5)..]
    fm_display = fm_content.each_line(chomp: false).map do |line|
      if (m = line.match(/\A([^:\n]+):\s*(.*)\n?\z/))
        "**#{m[1]}:**{.fmkey} #{m[2]}  \n"
      else
        line
      end
    end.join
    text = "\\-\\-\\-\n" + fm_display + "\\-\\-\\-\n\n" + rest
  end
end

fence_indent : Int32? = nil
fence_ticks  : Int32? = nil

# Single pass: plain-text output (no list conversion)
# Project names get bold + color; tasks get escaped dash; @tags handled by atag plugin
result = text.each_line(chomp: false).map do |line|
  if fence_indent.nil?
    if (m = line.match(/\A(\s*)(`{3,})/))
      fence_indent = m[1].size
      fence_ticks  = m[2].size
      next line
    end
  else
    if (m = line.match(/\A(\s*)(`{3,})\s*\z/)) &&
        m[1].size <= fence_indent.not_nil! && m[2].size >= fence_ticks.not_nil!
      fence_indent = nil
      fence_ticks  = nil
    end
    next line
  end

  if (m = line.match(/\A(\t*)[^\s#>].*:\s*\z/)) && !line.matches?(/https?:/)
    tabs   = m[1].size
    indent = "\u00A0\u00A0" * {tabs - 1, 0}.max
    cls    = "h4text"
    "#{indent}**#{line.sub(/\A\t*/, "").rstrip}**{.#{cls}}  \n"
  elsif line.matches?(/\A\t+- /)
    tabs   = line.index('-').not_nil!
    indent = "\u00A0\u00A0" * {tabs - 1, 0}.max
    "#{indent}\\- #{line.sub(/\A\t+- /, "").rstrip}  \n"
  elsif line.matches?(/\A {4}/)
    spaces = (line.chars.index { |c| c != ' ' } || line.size)
    level  = spaces // 4
    rest   = line.sub(/\A +/, "")
    rest.starts_with?("- ") \
      ? "#{"\u00A0\u00A0" * {level - 1, 0}.max}\\- #{rest[2..].rstrip}  \n" \
      : line
  elsif (m = line.chomp.match(/\A(##?) (.+)\z/))
    hashes  = m[1]
    content = m[2].rstrip
    prefix  = "\\#" * hashes.size
    "#{hashes} **#{prefix} #{content}**\n"
  elsif (m = line.chomp.match(/\A(####?) (.+)\z/))
    prefix = "\\#" * m[1].size
    "**#{prefix} #{m[2].rstrip}**{.h3text}  \n"
  else
    line
  end
end.join

print result
