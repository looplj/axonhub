export const USERS_QUERY = `
  query Users($first: Int, $after: Cursor) {
    users(first: $first, after: $after) {
      edges {
        node {
          id
          createdAt
          updatedAt
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