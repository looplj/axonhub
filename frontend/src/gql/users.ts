export const ME_QUERY = `
  query Me {
    me {
      email
      firstName
      lastName
      isOwner
      scopes
      preferLanguage
      avatar
      roles {
        code
        name
      }
    }
  }
`;

export const USERS_QUERY = `
  query Users($first: Int, $after: Cursor) {
    users(first: $first, after: $after) {
      edges {
        node {
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
          roles {
            edges {
              node {
                id
                name
              }
            }
          }
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
  query User($id: ID!) {
    user(id: $id) {
      id
      createdAt
      updatedAt
      email
      status
      firstName
      lastName
      isOwner
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
  }
`;

export const CREATE_USER_MUTATION = `
  mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
      id
      createdAt
      updatedAt
      email
      status
      firstName
      lastName
      isOwner
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
  }
`;

export const UPDATE_USER_MUTATION = `
  mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
    updateUser(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      email
      status
      firstName
      lastName
      isOwner
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
  }
`;

export const UPDATE_USER_STATUS_MUTATION = `
  mutation UpdateUserStatus($id: ID!, $status: UserStatus!) {
    updateUserStatus(id: $id, status: $status) {
      id
      createdAt
      updatedAt
      email
      status
      firstName
      lastName
      isOwner
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

export const INITIALIZE_SYSTEM_MUTATION = `
  mutation InitializeSystem($input: InitializeSystemInput!) {
    initializeSystem(input: $input) {
      success
      message
      user {
        id
        email
        firstName
        lastName
        isOwner
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