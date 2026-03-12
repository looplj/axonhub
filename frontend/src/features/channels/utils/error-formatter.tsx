import React from 'react';

/**
 * Formats a raw error message from the backend by extracting request IDs
 * and cleaning up the message.
 */
export function formatErrorMessage(error: string): {
  message: string;
  requestIDs: string[];
} {
  if (!error) return { message: '', requestIDs: [] };

  const requestIDs: string[] = [];
  const requestIdRegex = /\(request id: ([^)]+)\)/g;
  
  let match;
  let cleanedMessage = error;
  
  while ((match = requestIdRegex.exec(error)) !== null) {
    requestIDs.push(match[1]);
  }
  
  // Remove the (request id: ...) parts from the message
  cleanedMessage = error.replace(requestIdRegex, '').trim();
  
  // Clean up trailing commas or spaces that might remain
  cleanedMessage = cleanedMessage.replace(/,\s*$/, '').trim();

  return {
    message: cleanedMessage,
    requestIDs,
  };
}

/**
 * A component to display formatted error messages and request IDs.
 */
interface ErrorDisplayProps {
  error: string;
  className?: string;
  messageClassName?: string;
  requestIdClassName?: string;
}

export function ErrorDisplay({ 
  error, 
  className = '', 
  messageClassName = 'text-sm font-medium',
  requestIdClassName = 'text-xs text-muted-foreground mt-1 opacity-70'
}: ErrorDisplayProps) {
  const { message, requestIDs } = formatErrorMessage(error);

  if (!error) return null;

  return (
    <div className={className}>
      <div className={messageClassName}>{message}</div>
      {requestIDs.length > 0 && (
        <div className="mt-1 space-y-0.5">
          {requestIDs.map((id, index) => (
            <div key={`${id}-${index}`} className={requestIdClassName}>
              Request ID: {id}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
