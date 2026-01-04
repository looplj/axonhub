import { z } from 'zod';

/**
 * Shared password validation rules
 * Ensures consistent password requirements across authentication flows
 */
export const passwordValidation = {
  minLength: 8,
  pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)[a-zA-Z\d@$!%*?&]{8,}$/,
  messages: {
    required: 'auth.signIn.validation.passwordRequired',
    minLength: 'auth.signIn.validation.passwordMinLength',
    pattern: 'auth.signIn.validation.passwordPattern',
  },
};

/**
 * Enhanced password validation with stronger security requirements
 * - At least 8 characters
 * - At least one uppercase letter
 * - At least one lowercase letter
 * - At least one digit
 */
export const validatePassword = (password: string, t: (key: string) => string) => {
  if (!password) {
    return t(passwordValidation.messages.required);
  }

  if (password.length < passwordValidation.minLength) {
    return t(passwordValidation.messages.minLength);
  }

  if (!passwordValidation.pattern.test(password)) {
    return t(passwordValidation.messages.pattern);
  }

  return null;
};

/**
 * Zod schema for password validation
 */
export const passwordSchema = (t: (key: string) => string) =>
  z
    .string()
    .min(1, { message: t(passwordValidation.messages.required) })
    .min(passwordValidation.minLength, {
      message: t(passwordValidation.messages.minLength),
    });
// For the campatibility with the old version, we don't use the pattern.
// .regex(passwordValidation.pattern, {
//   message: t(passwordValidation.messages.pattern)
// })

/**
 * Password confirmation schema that ensures passwords match
 */
export const passwordConfirmationSchema = (t: (key: string) => string) =>
  z
    .object({
      password: passwordSchema(t),
      confirmPassword: z.string(),
    })
    .refine((data) => data.password === data.confirmPassword, {
      message: t('users.validation.passwordsNotMatch'),
      path: ['confirmPassword'],
    });
