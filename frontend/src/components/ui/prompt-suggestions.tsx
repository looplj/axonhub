interface PromptSuggestionsProps {
  label: string;
  append: (message: { role: 'user'; content: string }) => void;
  suggestions: string[];
}

export function PromptSuggestions({ label, append, suggestions }: PromptSuggestionsProps) {
  return (
    <div className='space-y-6'>
      <h2 className='text-center text-2xl font-bold'>{label}</h2>
      <div className='flex gap-6 text-sm'>
        {suggestions.map((suggestion) => (
          <button
            key={suggestion}
            onClick={() => append({ role: 'user', content: suggestion })}
            className='bg-background hover:bg-muted h-max flex-1 rounded-xl border p-4'
          >
            <p>{suggestion}</p>
          </button>
        ))}
      </div>
    </div>
  );
}
