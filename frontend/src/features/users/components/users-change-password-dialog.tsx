"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
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
import { User, ChangePasswordInput, changePasswordFormSchema } from "../data/schema";

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersChangePasswordDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const form = useForm({
    resolver: zodResolver(changePasswordFormSchema),
    defaultValues: {
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    },
  });

  const onSubmit = async (values: any) => {
    try {
      // 创建 API 输入，移除 confirmPassword
      const changePasswordInput: ChangePasswordInput = {
        currentPassword: values.currentPassword,
        newPassword: values.newPassword,
        // 注意：不包含 confirmPassword
      };
      
      // TODO: 实现修改密码的 API 调用
      console.log("Change password for user:", currentRow?.id, changePasswordInput);
      
      // 模拟 API 调用
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      toast.success(t("users.messages.passwordChangeSuccess"));
      form.reset();
      onOpenChange(false);
    } catch (error) {
      console.error("Failed to change password:", error);
      toast.error(t("users.messages.passwordChangeError"));
    }
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader className="text-left">
          <DialogTitle>{t("users.dialogs.changePassword.title")}</DialogTitle>
          <DialogDescription>
            {t("users.dialogs.changePassword.description", { 
              name: `${currentRow?.firstName} ${currentRow?.lastName}`, 
              email: currentRow?.email 
            })}
          </DialogDescription>
        </DialogHeader>
        
        <Form {...form}>
          <form
            id="change-password-form"
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4"
          >
            <FormField
              control={form.control}
              name="currentPassword"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users.form.currentPassword")}</FormLabel>
                  <FormControl>
                    <Input 
                      type="password" 
                      placeholder={t("users.form.currentPasswordPlaceholder")} 
                      {...field} 
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="newPassword"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users.form.newPassword")}</FormLabel>
                  <FormControl>
                    <Input 
                      type="password" 
                      placeholder={t("users.form.newPasswordPlaceholder")} 
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
                  <FormLabel>{t("users.form.confirmNewPassword")}</FormLabel>
                  <FormControl>
                    <Input 
                      type="password" 
                      placeholder={t("users.form.confirmNewPasswordPlaceholder")} 
                      {...field} 
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("users.buttons.cancel")}
          </Button>
          <Button
            type="submit"
            form="change-password-form"
            disabled={form.formState.isSubmitting}
          >
            {form.formState.isSubmitting ? t("users.buttons.changing") : t("users.buttons.changePassword")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}