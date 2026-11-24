export const ME_QUERY = `
  query Me {
    me {
      id
      email
      firstName
      lastName
      isOwner
      scopes
      preferLanguage
      avatar
      hasPassword
      oidcIdentities {
        id
        idpName
        issuer
        subject
        email
      }
      roles {
        name
      }
      projects {
        projectID
        isOwner
        scopes
        effectiveScopes
        roles {
          name
        }
      }
    }
  }
`;

export const ME_QUERY_OPERATION_NAME = 'Me';

const USER_FIELDS = `
  id
  createdAt
  updatedAt
  email
  status
  firstName
  lastName
  isOwner
  preferLanguage
  scopes
`;

const SYSTEM_ROLE_FIELDS = `
  roles(where: { level: system }) @include(if: $includeRoles) {
    edges {
      node {
        id
        name
      }
    }
  }
`;

export const USERS_QUERY = `
  query Users($first: Int, $after: Cursor, $orderBy: UserOrder, $where: UserWhereInput, $includeRoles: Boolean!) {
    users(first: $first, after: $after, orderBy: $orderBy, where: $where) {
      edges {
        node {
          ${USER_FIELDS}
          ${SYSTEM_ROLE_FIELDS}
        }
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
    }
  }
`;

export const USER_QUERY = `
  query User($id: ID!, $includeRoles: Boolean!) {
    user(id: $id) {
      ${USER_FIELDS}
      ${SYSTEM_ROLE_FIELDS}
    }
  }
`;

export const CREATE_USER_MUTATION = `
  mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
      ${USER_FIELDS}
    }
  }
`;

export const UPDATE_USER_MUTATION = `
  mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
    updateUser(id: $id, input: $input) {
      ${USER_FIELDS}
    }
  }
`;

export const UPDATE_USER_STATUS_MUTATION = `
  mutation UpdateUserStatus($id: ID!, $status: UserStatus!) {
    updateUserStatus(id: $id, status: $status) {
      ${USER_FIELDS}
    }
  }
`;

export const DELETE_USER_MUTATION = `
  mutation DeleteUser($id: ID!) {
    deleteUser(id: $id)
  }
`;

export const SIGN_IN_MUTATION = `
  mutation SignIn($input: SignInInput!) {
    signIn(input: $input) {
      user {
        id
        email
        firstName
        lastName
        isOwner
        preferLanguage
        scopes
        roles {
          edges {
            node {
              id
              name
            }
          }
        }
      }
      token
    }
  }
`;

export const SYSTEM_STATUS_QUERY = `
  query SystemStatus {
    systemStatus {
      isInitialized
    }
  }
`;

export const UPDATE_ME_MUTATION = `
  mutation UpdateMe($input: UpdateMeInput!) {
    updateMe(input: $input) {
      email
      firstName
      lastName
      isOwner
      preferLanguage
      avatar
    }
  }
`;

export const UPDATE_MY_PASSWORD_MUTATION = `
  mutation UpdateMyPassword($input: UpdateMyPasswordInput!) {
    updateMyPassword(input: $input)
  }
`;

export const UNLINK_OIDC_IDENTITY_MUTATION = `
  mutation UnlinkOIDCIdentity($id: ID!) {
    unlinkOIDCIdentity(id: $id)
  }
`;
