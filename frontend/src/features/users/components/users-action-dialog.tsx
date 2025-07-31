"use client";

import { useState, useEffect } from "react";
import { z } from "zod";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { X } from "lucide-react";
import { graphqlRequest } from "@/gql/graphql";
import { ROLES_QUERY, ALL_SCOPES_QUERY } from "@/gql/roles";
import { User, CreateUserInput, UpdateUserInput } from "../data/schema";
import { useCreateUser, useUpdateUser } from "../data/users";

// 统一的表单模式，包含所有字段
const formSchema = z.object({
  firstName: z.string().min(1, "First name is required"),
  lastName: z.string().min(1, "Last name is required"),
  email: z.string().email("Invalid email address"),
  password: z.string().optional(),
  confirmPassword: z.string().optional(),
  isOwner: z.boolean().optional(),
  roleIDs: z.array(z.string()).optional(),
  scopes: z.array(z.string()).optional(),
}).superRefine((data, ctx) => {
  // 只在创建用户且提供了密码时验证
  if (data.password || data.confirmPassword) {
    if (!data.password || data.password.length < 6) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Password must be at least 6 characters",
        path: ["password"],
      });
    }
    
    if (data.password !== data.confirmPassword) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Passwords don't match",
        path: ["confirmPassword"],
      });
    }
  }
});

type UserForm = z.infer<typeof formSchema>;

interface Role {
  id: string;
  name: string;
  description?: string;
  scopes?: string[];
}

interface ScopeInfo {
  scope: string;
  description?: string;
}

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersActionDialog({ currentRow, open, onOpenChange }: Props) {
  const isEdit = !!currentRow;
  const [roles, setRoles] = useState<Role[]>([]);
  const [allScopes, setAllScopes] = useState<ScopeInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const createUser = useCreateUser();
  const updateUser = useUpdateUser();

  // 根据是否为编辑模式使用不同的表单配置
  const form = useForm<UserForm>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          firstName: currentRow.firstName,
          lastName: currentRow.lastName,
          email: currentRow.email,
          password: "",
          confirmPassword: "",
          isOwner: currentRow.isOwner,
          roleIDs: currentRow.roles?.edges?.map(edge => edge.node.id) || [],
          scopes: currentRow.scopes || [],
        }
      : {
          firstName: "",
          lastName: "",
          email: "",
          password: "",
          confirmPassword: "",
          isOwner: false,
          roleIDs: [],
          scopes: [],
        },
  });

  // Load roles and scopes when dialog opens
  useEffect(() => {
    if (open) {
      loadRolesAndScopes();
    }
  }, [open]);

  const loadRolesAndScopes = async () => {
    setLoading(true);
    try {
      const [rolesData, scopesData] = await Promise.all([
        graphqlRequest(ROLES_QUERY, { first: 100 }),
        graphqlRequest(ALL_SCOPES_QUERY),
      ]);
      
      // Type the responses properly
      const rolesResponse = rolesData as {
        roles: {
          edges: Array<{
            node: {
              id: string;
              name: string;
              description?: string;
              scopes?: string[];
            };
          }>;
        };
      };
      
      const scopesResponse = scopesData as {
        allScopes: Array<{
          scope: string;
          description?: string;
        }>;
      };
      
      setRoles(rolesResponse.roles.edges.map(edge => edge.node));
      setAllScopes(scopesResponse.allScopes);
    } catch (error) {
      console.error("Failed to load roles and scopes:", error);
      toast.error("Failed to load roles and scopes");
    } finally {
      setLoading(false);
    }
  };

  const onSubmit = async (values: UserForm) => {
    try {
      if (isEdit && currentRow) {
        // For updates, we need to calculate role changes
        const currentRoleIDs = currentRow.roles?.edges?.map(edge => edge.node.id) || [];
        const newRoleIDs = values.roleIDs || [];
        
        const addRoleIDs = newRoleIDs.filter(id => !currentRoleIDs.includes(id));
        const removeRoleIDs = currentRoleIDs.filter(id => !newRoleIDs.includes(id));
        
        const updateInput: UpdateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          isOwner: values.isOwner,
          scopes: values.scopes,
        };
        
        // Only add role fields if there are changes
        if (addRoleIDs.length > 0) {
          updateInput.addRoleIDs = addRoleIDs;
        }
        if (removeRoleIDs.length > 0) {
          updateInput.removeRoleIDs = removeRoleIDs;
        }
        
        await updateUser.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        });
        toast.success("User updated successfully");
      } else {
        // 创建用户时，移除 confirmPassword 字段
        const createInput: CreateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          password: values.password || "",
          // 注意：不包含 confirmPassword
          isOwner: values.isOwner,
          scopes: values.scopes,
          roleIDs: values.roleIDs,
        };
        
        await createUser.mutateAsync(createInput);
        toast.success("User created successfully");
      }
      
      form.reset();
      onOpenChange(false);
    } catch (error) {
      console.error("Failed to save user:", error);
      toast.error("Failed to save user");
    }
  };

  const handleRoleToggle = (roleId: string) => {
    const currentRoles = form.getValues("roleIDs") || [];
    const newRoles = currentRoles.includes(roleId)
      ? currentRoles.filter(id => id !== roleId)
      : [...currentRoles, roleId];
    form.setValue("roleIDs", newRoles);
  };

  const handleScopeToggle = (scopeName: string) => {
    const currentScopes = form.getValues("scopes") || [];
    const newScopes = currentScopes.includes(scopeName)
      ? currentScopes.filter(name => name !== scopeName)
      : [...currentScopes, scopeName];
    form.setValue("scopes", newScopes);
  };

  const handleScopeRemove = (scopeName: string) => {
    const currentScopes = form.getValues("scopes") || [];
    const newScopes = currentScopes.filter(name => name !== scopeName);
    form.setValue("scopes", newScopes);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader className="text-left">
          <DialogTitle>{isEdit ? "Edit User" : "Add New User"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update the user here. " : "Create new user here. "}
            Click save when you're done.
          </DialogDescription>
        </DialogHeader>
        
        <div className="max-h-[60vh] overflow-y-auto">
          <Form {...form}>
            <form
              id="user-form"
              onSubmit={form.handleSubmit(onSubmit)}
              className="space-y-6"
            >
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="firstName"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>First Name</FormLabel>
                      <FormControl>
                        <Input placeholder="John" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="lastName"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Last Name</FormLabel>
                      <FormControl>
                        <Input placeholder="Doe" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Email</FormLabel>
                    <FormControl>
                      <Input placeholder="john.doe@example.com" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Password fields - only show when creating new user */}
              {!isEdit && (
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Password</FormLabel>
                        <FormControl>
                          <Input 
                            type="password" 
                            placeholder="Enter password" 
                            {...field} 
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="confirmPassword"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Confirm Password</FormLabel>
                        <FormControl>
                          <Input 
                            type="password" 
                            placeholder="Confirm password" 
                            {...field} 
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              <FormField
                control={form.control}
                name="isOwner"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0">
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>
                        Owner
                      </FormLabel>
                      <p className="text-sm text-muted-foreground">
                        Grant owner privileges to this user
                      </p>
                    </div>
                  </FormItem>
                )}
              />

              {/* Roles Section */}
              <div className="space-y-3">
                <FormLabel>Roles</FormLabel>
                {loading ? (
                  <div>Loading roles...</div>
                ) : (
                  <div className="grid grid-cols-2 gap-2">
                    {roles.map((role) => (
                      <div key={role.id} className="flex items-center space-x-2">
                        <Checkbox
                          id={`role-${role.id}`}
                          checked={(form.watch("roleIDs") || []).includes(role.id)}
                          onCheckedChange={() => handleRoleToggle(role.id)}
                        />
                        <label
                          htmlFor={`role-${role.id}`}
                          className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                        >
                          {role.name}
                        </label>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Scopes Section */}
              <div className="space-y-3">
                <FormLabel>Scopes</FormLabel>
                
                {/* Selected Scopes */}
                <div className="flex flex-wrap gap-2">
                  {(form.watch("scopes") || []).map((scope) => (
                    <Badge key={scope} variant="secondary" className="flex items-center gap-1">
                      {scope}
                      <X
                        className="h-3 w-3 cursor-pointer"
                        onClick={() => handleScopeRemove(scope)}
                      />
                    </Badge>
                  ))}
                </div>

                {/* Available Scopes */}
                {loading ? (
                  <div>Loading scopes...</div>
                ) : (
                  <div className="grid grid-cols-2 gap-2 max-h-32 overflow-y-auto border rounded p-2">
                    {allScopes.map((scope) => (
                      <div key={scope.scope} className="flex items-center space-x-2">
                        <Checkbox
                          id={`scope-${scope.scope}`}
                          checked={(form.watch("scopes") || []).includes(scope.scope)}
                          onCheckedChange={() => handleScopeToggle(scope.scope)}
                        />
                        <label
                          htmlFor={`scope-${scope.scope}`}
                          className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                        >
                          {scope.scope}
                        </label>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </form>
          </Form>
        </div>

        <DialogFooter>
          <Button
            type="submit"
            form="user-form"
            disabled={createUser.isPending || updateUser.isPending}
          >
            {createUser.isPending || updateUser.isPending ? "Saving..." : "Save changes"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}